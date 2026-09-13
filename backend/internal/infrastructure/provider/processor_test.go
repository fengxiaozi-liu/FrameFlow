package provider

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
)

func TestAllProviderKindsCompleteThroughWorker(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "provider-flow.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	configs := sqlite.NewProviderRepository(store)
	for _, capability := range []domain.Capability{domain.Story, domain.Image, domain.Video} {
		code := string(capability) + "-test"
		if err = configs.Save(domain.Config{Code: code, Name: code, Capability: capability, Model: "mock", BaseURL: "mock://local", Enabled: true, Status: domain.Healthy}); err != nil {
			t.Fatal(err)
		}
	}
	worker := queue.NewWorkerWithProcessor(store, NewGenerationProcessor(configs))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx)
	service := application.TaskService{Store: store, Worker: worker}
	for _, kind := range []string{"story", "image", "video"} {
		created, createErr := service.CreateWithProvider(kind, kind+"-test")
		if createErr != nil {
			t.Fatal(createErr)
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			current, _ := store.Get(created.ID)
			if current.Status == task.StatusSucceeded {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		current, _ := store.Get(created.ID)
		if current.Status != task.StatusSucceeded {
			t.Fatalf("%s task ended in %s: %s", kind, current.Status, current.Error)
		}
	}
}
