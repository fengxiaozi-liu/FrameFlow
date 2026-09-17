package main

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/processor"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	providerinfra "github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	ws "github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/websocket"
	"os"
	"path/filepath"
)

func InitRepo(ctx context.Context, path string) (*sqlite.TaskRepository, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, err
	}
	return sqlite.Open(ctx, path)
}
func InitService(store *sqlite.TaskRepository, vault provider.CredentialVault, uploadDir string, connections *ws.Hub) (application.TaskService, application.ProjectService, application.ProviderService, application.MaterialService) {
	providers := application.ProviderService{Repo: sqlite.NewProviderRepository(store), Vault: vault}
	return application.TaskService{Store: store, Connections: connections, Providers: providers}, application.ProjectService{Repo: sqlite.NewProjectRepository(store)}, providers, application.MaterialService{Repo: sqlite.NewMaterialRepository(store), UploadDir: uploadDir}
}
func InitProcessor(store *sqlite.TaskRepository, configs provider.Repository, connections *ws.Hub) (*queue.Worker, *processor.Processor) {
	worker := queue.NewWorker()
	p := processor.New(configs, providerinfra.NewRegistry(), worker, store)
	p.Connections = connections
	return worker, p
}
