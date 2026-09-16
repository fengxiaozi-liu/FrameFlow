package processor

import (
	"context"
	"testing"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type providerRepository struct {
	config provider.Config
}

func (r providerRepository) Save(context.Context, provider.Config) error {
	return nil
}

func (r providerRepository) Get(context.Context, string) (provider.Config, error) {
	return r.config, nil
}

func (r providerRepository) List(context.Context, provider.Capability) ([]provider.Config, error) {
	return []provider.Config{r.config}, nil
}

func (r providerRepository) Delete(context.Context, string) error {
	return nil
}

type videoGenerator struct {
	request provider.VideoRequest
}

func (g *videoGenerator) Generate(_ context.Context, request provider.VideoRequest, _ provider.RequestOptions) (provider.VideoResult, error) {
	g.request = request
	return provider.VideoResult{JobReference: "job-1"}, nil
}

func TestProcessorForwardsTaskInputToProvider(t *testing.T) {
	config := provider.Config{Code: "video", Vendor: "test", Capability: provider.Video, Model: "video-model", Enabled: true, Status: provider.Healthy}
	repository := providerRepository{config: config}
	registry := provider.NewRegistry()
	generator := &videoGenerator{}
	registry.Register("test", func(context.Context, provider.Config) (provider.Adapter, error) {
		return generator, nil
	})
	processor := New(repository, registry, nil, nil)
	input := task.Input{Prompt: "城市夜景", AspectRatio: "16:9", SourceImageURL: "/media/start.png"}
	item := task.New("task-1", task.KindVideo, input, time.Now())
	item.ProviderCode = config.Code

	if err := processor.Process(context.Background(), item, func(int, string) {}); err != nil {
		t.Fatal(err)
	}
	if generator.request.Prompt != input.Prompt || generator.request.Model != config.Model || generator.request.AspectRatio != input.AspectRatio || generator.request.SourceImageURL != input.SourceImageURL {
		t.Fatalf("provider request does not contain task input: %+v", generator.request)
	}
}
