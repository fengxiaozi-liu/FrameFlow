package processor

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
)

type recordingClient struct{ request provider.VideoRequest }

func (*recordingClient) GenerateText(context.Context, provider.Connection, provider.Model, provider.StoryRequest, provider.RequestOptions) (provider.StoryResult, error) {
	return provider.StoryResult{}, provider.ErrUnsupported
}
func (*recordingClient) GenerateImage(context.Context, provider.Connection, provider.Model, provider.ImageRequest, provider.RequestOptions) (provider.ImageResult, error) {
	return provider.ImageResult{}, provider.ErrUnsupported
}
func (c *recordingClient) GenerateVideo(_ context.Context, _ provider.Connection, _ provider.Model, input provider.VideoRequest, _ provider.RequestOptions) (provider.VideoResult, error) {
	c.request = input
	return provider.VideoResult{URL: "https://example.com/video.mp4"}, nil
}
func (*recordingClient) PollVideo(context.Context, provider.Connection, provider.Model, string) (provider.VideoResult, bool, error) {
	return provider.VideoResult{}, false, provider.ErrUnsupported
}

func TestProcessorForwardsTaskInputToModelClient(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "processor.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	connection := provider.Connection{ID: "test", Name: "Test", Vendor: "bailian", BaseURL: "https://example.com"}
	model := provider.Model{ID: "test:video", ConnectionID: connection.ID, RemoteID: "wan2.7-i2v", Capabilities: []provider.Capability{provider.Video}, Supported: true, Enabled: true}
	if err := store.SaveConnection(ctx, connection); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveModel(ctx, model); err != nil {
		t.Fatal(err)
	}
	client := &recordingClient{}
	p := New(nil, store)
	p.Models, p.ProviderConnections, p.ModelClient = store, store, client
	input := task.Input{Prompt: "city at night", AspectRatio: "16:9", SourceImageURL: "https://example.com/start.png"}
	item := task.New("task-1", task.KindVideo, input, time.Now())
	item.ModelID = model.ID
	if err := p.processModel(ctx, &item, func(int, task.Stage) {}); err != nil {
		t.Fatal(err)
	}
	if client.request.Prompt != input.Prompt || client.request.Model != model.RemoteID || client.request.AspectRatio != input.AspectRatio || client.request.SourceImageURL != input.SourceImageURL {
		t.Fatalf("model request does not contain task input: %+v", client.request)
	}
}
