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
	projects := sqlite.NewProjectRepository(store)
	providers.Catalog = application.CatalogService{Connections: store, Models: store, Tasks: store, Vault: vault, Discovery: providerinfra.NewDiscovery()}
	return application.TaskService{Store: store, UploadDir: uploadDir, Connections: connections, Providers: providers, Projects: projects}, application.ProjectService{Repo: projects}, providers, application.MaterialService{Repo: sqlite.NewMaterialRepository(store), UploadDir: uploadDir}
}
func InitProcessor(store *sqlite.TaskRepository, connections *ws.Hub) (*queue.Worker, *processor.Processor) {
	worker := queue.NewConcurrentWorker(3)
	p := processor.New(worker, store)
	p.Connections = connections
	return worker, p
}
