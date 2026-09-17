package processor

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type failingGenerator struct {
	recordingClient
	calls  int
	during func()
}

func (g *failingGenerator) GenerateVideo(context.Context, provider.Connection, provider.Model, provider.VideoRequest, provider.RequestOptions) (provider.VideoResult, error) {
	g.calls++
	if g.during != nil {
		g.during()
	}
	return provider.VideoResult{}, errors.New("generation failed")
}

func TestExecuteOwnsRetriesAndPreservesCancellation(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		name := "retry exhaustion"
		if cancel {
			name = "cancel during generation"
		}
		t.Run(name, func(t *testing.T) {
			store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "execute.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			connection := provider.Connection{ID: "test", Name: "Test", Vendor: "bailian", BaseURL: "https://example.com"}
			model := provider.Model{ID: "test:video", ConnectionID: connection.ID, RemoteID: "wan2.7-i2v", Capabilities: []provider.Capability{provider.Video}, Supported: true, Enabled: true}
			if err := store.SaveConnection(context.Background(), connection); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveModel(context.Background(), model); err != nil {
				t.Fatal(err)
			}
			generator := &failingGenerator{}
			p := New(nil, store)
			p.Models, p.ProviderConnections, p.ModelClient = store, store, generator
			item := task.New("task", task.KindVideo, task.Input{Prompt: "test"}, time.Now())
			item.ModelID = model.ID
			if err := store.Save(context.Background(), item); err != nil {
				t.Fatal(err)
			}
			if cancel {
				generator.during = func() {
					current, _ := store.Get(context.Background(), item.ID)
					_ = current.Cancel(time.Now())
					_ = store.Save(context.Background(), current)
				}
			}
			p.Execute(context.Background(), item)
			got, _ := store.Get(context.Background(), item.ID)
			if cancel {
				if got.Status != task.StatusCancelled || generator.calls != 1 {
					t.Fatalf("cancel overwritten: %+v, calls=%d", got, generator.calls)
				}
			} else if got.Status != task.StatusFailed || got.RetryCount != 3 || generator.calls != 3 {
				t.Fatalf("retry policy failed: %+v, calls=%d", got, generator.calls)
			}
		})
	}
}

func TestHistoricalProviderTaskFailsWithoutFactoryRegistry(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	item := task.New("legacy", task.KindStory, task.Input{Prompt: "test"}, time.Now())
	item.ProviderCode = "old-provider"
	if err := store.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	p := New(nil, store)
	p.Execute(ctx, item)
	got, err := store.Get(ctx, item.ID)
	if err != nil || got.Status != task.StatusFailed || !strings.Contains(got.Error, "select a model") {
		t.Fatalf("historical task: %+v, %v", got, err)
	}
}
