package application

import (
	"context"
	"testing"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type submissionModelRepository struct {
	provider.ModelRepository
	model provider.Model
}

func (r *submissionModelRepository) GetModel(context.Context, string) (provider.Model, error) {
	return r.model, nil
}

func TestSubmissionFreezesConnectionAndModelWithoutDirectoryDemo(t *testing.T) {
	m := provider.MigrateLegacyModel(provider.Model{ID: "m", ConnectionID: "c", RemoteID: "qwen-plus", Capabilities: []provider.Capability{provider.Story}, Enabled: true})
	models := &submissionModelRepository{model: m}
	profile := provider.DefaultCustomDiscoveryProfile()
	connections := &configurationRepository{exists: true, current: provider.Connection{ID: "c", Name: "Custom", Vendor: "custom", BaseURL: "https://original.example.com", Revision: 3, Discovery: &profile, Auth: &provider.ConnectionAuth{Mode: "bearer"}}}
	item := task.Task{Kind: task.KindStory, ModelID: "m"}
	if err := freezeTaskModel(context.Background(), models, &item, connections); err != nil {
		t.Fatal(err)
	}
	connections.current.BaseURL = "https://changed.example.com"
	connections.current.Auth.Mode = "none"
	models.model.Operations[provider.Story] = provider.OperationBinding{}
	if item.ConnectionConfig.BaseURL != "https://original.example.com" || item.ConnectionConfig.Auth.Mode != "bearer" || item.ConnectionConfig.Revision != 3 || item.ConnectionConfig.Discovery != nil || item.ModelConfig.Operations[provider.Story].Protocol != provider.ChatProtocol {
		t.Fatal("submission snapshot was mutable or contained discovery demo")
	}
}
