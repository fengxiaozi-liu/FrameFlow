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

func (g waitingVideo) GenerateVideo(ctx context.Context, _ domain.Connection, _ domain.Model, _ domain.VideoRequest, _ domain.RequestOptions) (domain.VideoResult, error) {
	g.entered <- ctx
	<-ctx.Done()
	close(g.stopped)
	return domain.VideoResult{}, ctx.Err()
}
func (waitingVideo) GenerateText(context.Context, domain.Connection, domain.Model, domain.StoryRequest, domain.RequestOptions) (domain.StoryResult, error) {
	return domain.StoryResult{}, domain.ErrUnsupported
}
func (waitingVideo) GenerateImage(context.Context, domain.Connection, domain.Model, domain.ImageRequest, domain.RequestOptions) (domain.ImageResult, error) {
	return domain.ImageResult{}, domain.ErrUnsupported
}
func (waitingVideo) PollVideo(context.Context, domain.Connection, domain.Model, string) (domain.VideoResult, bool, error) {
	return domain.VideoResult{}, false, domain.ErrUnsupported
}

func TestTaskOutlivesRequestAndCancellationStopsProvider(t *testing.T) {
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "cancel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	configs := sqlite.NewProviderRepository(store)
	connection := domain.Connection{ID: "test", Name: "Test", Vendor: "bailian", BaseURL: "https://example.com"}
	model := domain.Model{ID: "test:blocking", ConnectionID: connection.ID, RemoteID: "wan2.7-i2v", Capabilities: []domain.Capability{domain.Video}, Supported: true, Enabled: true}
	if err := store.SaveConnection(context.Background(), connection); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveModel(context.Background(), model); err != nil {
		t.Fatal(err)
	}
	generator := waitingVideo{entered: make(chan context.Context, 1), stopped: make(chan struct{})}
	processor := domainprocessor.New(queue.NewWorker(), store)
	processor.Models, processor.ProviderConnections, processor.ModelClient = store, store, generator
	service := application.TaskService{Store: store, Enqueuer: processor, Providers: application.ProviderService{Repo: configs, Catalog: application.CatalogService{Models: store}}}
	requestCtx, endRequest := context.WithCancel(context.Background())
	item := createTask(t, service, requestCtx, "video", "blocking")
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
	connection := domain.Connection{ID: "test", Name: "Test", Vendor: "bailian", BaseURL: "https://example.com"}
	if err := store.SaveConnection(context.Background(), connection); err != nil {
		t.Fatal(err)
	}
	for _, capability := range []domain.Capability{domain.Story, domain.Image, domain.Video} {
		code := string(capability) + "-test"
		if err = store.SaveModel(context.Background(), domain.Model{ID: "test:" + code, ConnectionID: connection.ID, RemoteID: code, Capabilities: []domain.Capability{capability}, Supported: true, Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	worker := queue.NewWorker()
	processor := domainprocessor.New(worker, store)
	processor.Models, processor.ProviderConnections, processor.ModelClient = store, store, testClient{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processor.Start(ctx)
	service := application.TaskService{Store: store, Enqueuer: processor, Providers: application.ProviderService{Repo: configs, Catalog: application.CatalogService{Models: store}}}
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

type testClient struct{}

func (testClient) GenerateText(_ context.Context, _ domain.Connection, _ domain.Model, r domain.StoryRequest, _ domain.RequestOptions) (domain.StoryResult, error) {
	return domain.StoryResult{Document: r.Prompt}, nil
}
func (testClient) GenerateImage(context.Context, domain.Connection, domain.Model, domain.ImageRequest, domain.RequestOptions) (domain.ImageResult, error) {
	return domain.ImageResult{URL: "https://example.com/image.png"}, nil
}
func (testClient) GenerateVideo(context.Context, domain.Connection, domain.Model, domain.VideoRequest, domain.RequestOptions) (domain.VideoResult, error) {
	return domain.VideoResult{URL: "https://example.com/video.mp4"}, nil
}
func (testClient) PollVideo(context.Context, domain.Connection, domain.Model, string) (domain.VideoResult, bool, error) {
	return domain.VideoResult{}, false, domain.ErrUnsupported
}

func createTask(t *testing.T, service application.TaskService, ctx context.Context, kind, code string) task.Task {
	t.Helper()
	router := gin.New()
	router.POST("/tasks", service.Create)
	body, _ := json.Marshal(map[string]string{"kind": kind, "model_id": "test:" + code, "prompt": "test", "source_image_url": "https://example.com/start.png"})
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
