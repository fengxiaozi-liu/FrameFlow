package main

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	providerinfra "github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	securityinfra "github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/security"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	httpapi "github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/http"
	"github.com/fengxiaozi-liu/FrameFlow/internal/web"
	"io/fs"
	"log"
	"net/http"
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
	baseDir := "."
	if openOnStart == "1" {
		if executable, pathErr := os.Executable(); pathErr == nil {
			baseDir = filepath.Dir(executable)
		}
	}
	databasePath := os.Getenv("FRAMEFLOW_DATABASE_PATH")
	if databasePath == "" {
		databasePath = filepath.Join(baseDir, "data", "frameflow.db")
	}
	if err := os.MkdirAll(filepath.Dir(databasePath), 0750); err != nil {
		log.Fatal(err)
	}
	store, err := sqlite.Open(databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	providerRepo := sqlite.NewProviderRepository(store)
	worker := queue.NewWorkerWithProcessor(store, providerinfra.NewGenerationProcessor(providerRepo))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx)
	tasks := application.TaskService{Store: store, Worker: worker}
	providers := application.ProviderService{Repo: providerRepo}
	uploadDir := filepath.Join(baseDir, "data", "uploads")
	vault, err := securityinfra.OpenVault(filepath.Join(baseDir, "data", "secrets"))
	if err != nil {
		log.Fatal(err)
	}
	staticFiles, _ := fs.Sub(web.Files, "dist")
	server := httpapi.Server{Store: store, Worker: worker, TaskService: tasks, Projects: application.ProjectService{Repo: sqlite.NewProjectRepository(store)}, Providers: providers, Materials: application.MaterialService{Repo: sqlite.NewMaterialRepository(store)}, Generation: application.GenerationService{Tasks: tasks, Providers: providers}, AuthToken: os.Getenv("FRAMEFLOW_API_TOKEN"), UploadDir: uploadDir, Vault: vault, StaticFS: staticFiles}
	httpServer := &http.Server{Addr: ":8080", Handler: server.Routes(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("FrameFlow listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	if openOnStart == "1" {
		go func() {
			time.Sleep(500 * time.Millisecond)
			var command string
			var args []string
			switch runtime.GOOS {
			case "windows":
				command, args = "rundll32", []string{"url.dll,FileProtocolHandler", "http://127.0.0.1:8080"}
			case "darwin":
				command, args = "open", []string{"http://127.0.0.1:8080"}
			default:
				command, args = "xdg-open", []string{"http://127.0.0.1:8080"}
			}
			_ = exec.Command(command, args...).Start()
		}()
	}
	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-stopCtx.Done()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
