package main

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	securityinfra "github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/security"
	httpapi "github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/http"
	ws "github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/websocket"
	"github.com/fengxiaozi-liu/FrameFlow/internal/web"
	"github.com/gin-gonic/gin"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

var openOnStart = "0"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	baseDir := "."
	if openOnStart == "1" {
		if executable, pathErr := os.Executable(); pathErr == nil {
			baseDir = filepath.Dir(executable)
		}
		logPath := filepath.Join(baseDir, "data", "frameflow.log")
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
		return
	}
	defer store.Close()
	uploadDir := filepath.Join(baseDir, "data", "uploads")
	vault, err := securityinfra.OpenVault(ctx, filepath.Join(baseDir, "data", "secrets"))
	if err != nil {
		log.Print(err)
		return
	}
	tasks, projects, providers, materials := InitService(store, vault, uploadDir, ws.Connections)
	worker, processor := InitProcessor(store, providers.Repo, ws.Connections)
	tasks.Enqueuer = processor
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); processor.Start(ctx) }()
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
	router := gin.Default()
	httpapi.UseMiddleware(router, os.Getenv("FRAMEFLOW_API_TOKEN"), 0)
	httpapi.RegisterRoutes(router, tasks, projects, providers, materials, application.SystemService{Tasks: tasks, Projects: projects, Providers: providers, Materials: materials, Shutdown: shutdown, QueueDepth: worker.Depth})
	router.GET("/ws", ws.Handler)
	httpapi.RegisterStatic(router, uploadDir, staticFiles)
	serveError := make(chan error, 1)
	go func() { log.Printf("FrameFlow listening on %s", address); serveError <- router.Run(address) }()
	if openOnStart == "1" {
		go func() {
			time.Sleep(500 * time.Millisecond)
			var command string
			var args []string
			switch runtime.GOOS {
			case "windows":
				command, args = "rundll32", []string{"url.dll,FileProtocolHandler", "http://" + address}
			case "darwin":
				command, args = "open", []string{"http://" + address}
			default:
				command, args = "xdg-open", []string{"http://" + address}
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
