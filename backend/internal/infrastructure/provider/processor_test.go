package provider

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	domainprocessor "github.com/fengxiaozi-liu/FrameFlow/internal/domain/processor"
	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
)

type waitingVideo struct {
	entered chan context.Context
	stopped chan struct{}
}

func (g waitingVideo) Generate(ctx context.Context, _ domain.VideoRequest, _ domain.RequestOptions) (domain.VideoResult, error) {
	g.entered <- ctx
	<-ctx.Done()
	close(g.stopped)
	return domain.VideoResult{}, ctx.Err()
}

func TestTaskOutlivesRequestAndCancellationStopsProvider(t *testing.T) {
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "cancel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	configs := sqlite.NewProviderRepository(store)
	config := domain.Config{Code: "blocking", Vendor: "test", Capability: domain.Video, Enabled: true, Status: domain.Healthy}
	if err := configs.Save(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	generator := waitingVideo{entered: make(chan context.Context, 1), stopped: make(chan struct{})}
	registry := domain.NewRegistry()
	registry.Register("test", func(context.Context, domain.Config) (domain.Adapter, error) { return generator, nil })
	processor := domainprocessor.New(configs, registry, queue.NewWorker(), store)
	service := application.TaskService{Store: store, Enqueuer: processor, Providers: application.ProviderService{Repo: configs}}
	requestCtx, endRequest := context.WithCancel(context.Background())
	item := createTask(t, service, requestCtx, "video", config.Code)
	endRequest()
	lifecycle, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); processor.Start(lifecycle) }()
	defer func() { stop(); <-done }()
	select {
	case ctx := <-generator.entered:
		if ctx.Err() != nil {
			t.Fatal("request cancellation leaked into task", ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}
	router := gin.New()
	router.POST("/tasks/:id/cancel", service.Cancel)
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest("POST", "/tasks/"+item.ID+"/cancel", nil))
	if out.Code != 200 {
		t.Fatal(out.Code, out.Body.String())
	}
	select {
	case <-generator.stopped:
	case <-time.After(time.Second):
		t.Fatal("task cancellation did not stop provider")
	}
	stop()
	<-done
	current, err := store.Get(context.Background(), item.ID)
	if err != nil || current.Status != task.StatusCancelled {
		t.Fatal(current, err)
	}
}

func TestAllProviderKindsCompleteThroughWorker(t *testing.T) {
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "provider-flow.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	configs := sqlite.NewProviderRepository(store)
	for _, capability := range []domain.Capability{domain.Story, domain.Image, domain.Video} {
		code := string(capability) + "-test"
		if err = configs.Save(context.Background(), domain.Config{Code: code, Name: code, Capability: capability, Model: "mock", BaseURL: "mock://local", Enabled: true, Status: domain.Healthy}); err != nil {
			t.Fatal(err)
		}
	}
	worker := queue.NewWorker()
	processor := domainprocessor.New(configs, NewRegistry(), worker, store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Start(ctx)
	service := application.TaskService{Store: store, Enqueuer: processor, Providers: application.ProviderService{Repo: configs}}
	for _, kind := range []string{"story", "image", "video"} {
		created := createTask(t, service, context.Background(), kind, kind+"-test")
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			current, _ := store.Get(context.Background(), created.ID)
			if current.Status == task.StatusSucceeded {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		current, _ := store.Get(context.Background(), created.ID)
		if current.Status != task.StatusSucceeded {
			t.Fatalf("%s task ended in %s: %s", kind, current.Status, current.Error)
		}
	}
}

func createTask(t *testing.T, service application.TaskService, ctx context.Context, kind, code string) task.Task {
	t.Helper()
	router := gin.New()
	router.POST("/tasks", service.Create)
	body, _ := json.Marshal(map[string]string{"kind": kind, "provider_code": code, "prompt": "test"})
	out := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/tasks", strings.NewReader(string(body))).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(out, req)
	if out.Code != 202 {
		t.Fatalf("create: %d %s", out.Code, out.Body.String())
	}
	var item task.Task
	if err := json.Unmarshal(out.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	return item
}
