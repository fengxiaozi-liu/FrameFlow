package processor

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"testing"
	"time"
)

type failingGenerator struct {
	calls  int
	during func()
}

func (g *failingGenerator) Generate(context.Context, provider.VideoRequest, provider.RequestOptions) (provider.VideoResult, error) {
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
			store := queue.NewStore()
			config := provider.Config{Code: "video", Vendor: "test", Capability: provider.Video, Enabled: true, Status: provider.Healthy}
			registry := provider.NewRegistry()
			generator := &failingGenerator{}
			registry.Register("test", func(context.Context, provider.Config) (provider.Adapter, error) { return generator, nil })
			p := New(providerRepository{config: config}, registry, nil, store)
			item := task.New("task", task.KindVideo, task.Input{Prompt: "test"}, time.Now())
			item.ProviderCode = config.Code
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
