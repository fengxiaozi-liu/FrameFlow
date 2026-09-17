package main

import (
	"context"
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	providerinfra "github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider"
	securityinfra "github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/security"
	httpapi "github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/http"
	ws "github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/websocket"
	"github.com/fengxiaozi-liu/FrameFlow/internal/web"
	"github.com/gin-gonic/gin"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

var openOnStart = "0"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	baseDir := "."
	if openOnStart == "1" {
		minimizeConsole()
		gin.SetMode(gin.ReleaseMode)
		if executable, pathErr := os.Executable(); pathErr == nil {
			baseDir = filepath.Dir(executable)
		}
	}
	logPath := filepath.Join(baseDir, "data", "frameflow.log")
	if openOnStart == "1" {
		if err := os.MkdirAll(filepath.Dir(logPath), 0750); err == nil {
			if file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640); err == nil {
				defer file.Close()
				log.SetOutput(file)
			}
		}
	}
	databasePath := os.Getenv("FRAMEFLOW_DATABASE_PATH")
	if databasePath == "" {
		databasePath = filepath.Join(baseDir, "data", "frameflow.db")
	}
	store, err := InitRepo(ctx, databasePath)
	if err != nil {
		log.Print(err)
		startupError(err, logPath)
		return
	}
	defer store.Close()
	uploadDir := filepath.Join(baseDir, "data", "uploads")
	if configured := os.Getenv("FRAMEFLOW_UPLOAD_DIR"); configured != "" {
		uploadDir = configured
	}
	vaultDir := os.Getenv("FRAMEFLOW_VAULT_DIR")
	if vaultDir == "" {
		vaultDir = filepath.Join(baseDir, "data", "secrets")
	}
	vault, err := securityinfra.OpenVault(ctx, vaultDir)
	if err != nil {
		log.Print(err)
		startupError(err, logPath)
		return
	}
	if err := migrateProviders(ctx, store, vault); err != nil {
		log.Print("migrate provider configuration: ", err)
		startupError(err, logPath)
		return
	}
	tasks, projects, providers, materials := InitService(store, vault, uploadDir, ws.Connections)
	worker, processor := InitProcessor(store, ws.Connections)
	processor.Models = store
	processor.ProviderConnections = store
	processor.ModelClient = providerinfra.NewRegistry(vault)
	processor.SaveResult = (providerinfra.ResultStore{Directory: uploadDir}).Save
	processor.MediaDir = uploadDir
	tasks.Enqueuer = processor
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); processor.Start(ctx) }()
	go func() {
		if err := recoverModelTasks(ctx, store, processor.Enqueue); err != nil && ctx.Err() == nil {
			log.Print("recover model tasks: ", err)
		}
	}()
	staticFiles, _ := fs.Sub(web.Files, "dist")
	address := os.Getenv("FRAMEFLOW_ADDRESS")
	if address == "" {
		address = "127.0.0.1:28741"
	}
	shutdownRequested := make(chan struct{}, 1)
	shutdown := func() {
		select {
		case shutdownRequested <- struct{}{}:
		default:
		}
	}
	var router *gin.Engine
	if openOnStart == "1" {
		router = gin.New()
		router.Use(gin.RecoveryWithWriter(log.Writer()))
	} else {
		router = gin.Default()
	}
	rateLimit, _ := strconv.Atoi(os.Getenv("FRAMEFLOW_RATE_LIMIT"))
	httpapi.UseMiddleware(router, os.Getenv("FRAMEFLOW_API_TOKEN"), rateLimit)
	httpapi.RegisterRoutes(router, tasks, projects, providers, materials, application.SystemService{Tasks: tasks, Projects: projects, Providers: providers, Materials: materials, Shutdown: shutdown, QueueDepth: worker.Depth})
	router.GET("/ws", ws.Handler)
	httpapi.RegisterStatic(router, uploadDir, staticFiles)
	serveError := make(chan error, 1)
	listener, listenErr := net.Listen("tcp", address)
	if listenErr != nil {
		if openOnStart == "1" {
			startupError(listenErr, logPath)
		}
		serveError <- listenErr
	} else {
		log.Printf("FrameFlow listening on %s", listener.Addr())
		if openOnStart == "1" {
			printConsoleStatus(os.Stdout, listener.Addr().String(), logPath)
		}
		go func() { serveError <- http.Serve(listener, router) }()
	}
	if openOnStart == "1" && listenErr == nil {
		go func() {
			time.Sleep(500 * time.Millisecond)
			var command string
			var args []string
			switch runtime.GOOS {
			case "windows":
				command, args = "rundll32", []string{"url.dll,FileProtocolHandler", "http://" + listener.Addr().String()}
			case "darwin":
				command, args = "open", []string{"http://" + listener.Addr().String()}
			default:
				command, args = "xdg-open", []string{"http://" + listener.Addr().String()}
			}
			_ = exec.Command(command, args...).Start()
		}()
	}
	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-stopCtx.Done():
	case <-shutdownRequested:
	case err := <-serveError:
		log.Printf("HTTP server: %v", err)
	}
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	cancel()
	select {
	case <-workerDone:
	case <-shutdownCtx.Done():
		log.Printf("worker shutdown: %v", shutdownCtx.Err())
	}
	ws.CloseAll()
}

func startupError(err error, logPath string) {
	if openOnStart != "1" {
		return
	}
	fmt.Fprintf(os.Stdout, "\n  FrameFlow  |  启动失败\n  原因: %v\n  日志: %s\n", err, logPath)
	notifyStartupError(err, logPath)
}
