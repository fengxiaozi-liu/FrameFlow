package http

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	domainprocessor "github.com/fengxiaozi-liu/FrameFlow/internal/domain/processor"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	ws "github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/websocket"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testServer(t *testing.T) (testServices, func()) {
	db, e := sqlite.Open(context.Background(), ":memory:")
	if e != nil {
		t.Fatal(e)
	}
	worker := queue.NewWorker()
	projectRepo := sqlite.NewProjectRepository(db)
	testProject, _ := project.New("test-project", "Test", time.Now().UTC())
	_ = testProject.AddDraft(project.Draft{ID: "test-draft", Name: "Draft"})
	if err := projectRepo.Save(context.Background(), testProject); err != nil {
		t.Fatal(err)
	}
	providers := application.ProviderService{Repo: sqlite.NewProviderRepository(db)}
	providers.Catalog = application.CatalogService{Connections: db, Models: db}
	conn := provider.Connection{ID: "test-connection", Name: "Test", Vendor: "bailian", BaseURL: "https://dashscope.aliyuncs.com"}
	if err := db.SaveConnection(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	for _, cap := range []provider.Capability{provider.Story, provider.Image, provider.Video} {
		model := provider.Model{ID: provider.ModelID(conn.ID, string(cap)), ConnectionID: conn.ID, RemoteID: string(cap), Name: string(cap), Capabilities: []provider.Capability{cap}, Supported: true, Enabled: true, Default: true}
		if err := db.SaveModel(context.Background(), model); err != nil {
			t.Fatal(err)
		}
	}
	processor := domainprocessor.New(worker, db)
	processor.Models, processor.ProviderConnections, processor.ModelClient = db, db, testModelClient{}
	processor.Results = application.TaskResultApplier{Projects: projectRepo}
	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); processor.Start(ctx) }()
	tasks := application.TaskService{Store: db, Enqueuer: processor, Connections: ws.Connections, Providers: providers, Projects: projectRepo}
	return testServices{Worker: worker, TaskService: tasks, Projects: application.ProjectService{Repo: projectRepo}, Providers: providers, Materials: application.MaterialService{Repo: sqlite.NewMaterialRepository(db)}}, func() { cancel(); <-workerDone; ws.CloseAll(); _ = db.Close() }
}

type testModelClient struct{}

func (testModelClient) GenerateText(_ context.Context, _ provider.Connection, _ provider.Model, r provider.StoryRequest, _ provider.RequestOptions) (provider.StoryResult, error) {
	return provider.StoryResult{Document: r.Prompt}, nil
}
func (testModelClient) GenerateImage(context.Context, provider.Connection, provider.Model, provider.ImageRequest, provider.RequestOptions) (provider.ImageResult, error) {
	return provider.ImageResult{URL: "https://example.com/image.png"}, nil
}
func (testModelClient) GenerateVideo(context.Context, provider.Connection, provider.Model, provider.VideoRequest, provider.RequestOptions) (provider.VideoResult, error) {
	return provider.VideoResult{URL: "https://example.com/video.mp4"}, nil
}
func (testModelClient) PollVideo(context.Context, provider.Connection, provider.Model, string) (provider.VideoResult, bool, error) {
	return provider.VideoResult{}, false, provider.ErrUnsupported
}
func testProcessor(s testServices) *domainprocessor.Processor {
	p := domainprocessor.New(queue.NewWorker(), s.TaskService.Store)
	p.Models, p.ProviderConnections, p.ModelClient = s.Providers.Catalog.Models, s.Providers.Catalog.Connections, testModelClient{}
	p.Results = application.TaskResultApplier{Projects: s.Projects.Repo}
	return p
}

func request(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestAPICoreFlow(t *testing.T) {
	s, done := testServer(t)
	defer done()
	h := testRouter(s)
	legacy := request(t, h, "POST", "/api/tasks", `{"kind":"story","provider_code":"legacy","prompt":"test"}`)
	if legacy.Code != http.StatusBadRequest {
		t.Fatal(legacy.Code, legacy.Body.String())
	}
	w := request(t, h, "POST", "/api/projects", `{"name":"Demo"}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var p map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	id := p["id"].(string)
	w = request(t, h, "POST", "/api/projects/"+id+"/drafts", `{"id":"d1","name":"Draft"}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(t, h, "POST", "/api/materials", `{"name":"Cover","kind":"visual","url":"/media/cover.png"}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(t, h, "POST", "/api/tasks", `{"project_id":"`+id+`","draft_id":"d1","kind":"video","prompt":"测试视频","source_image_url":"https://example.com/frame.png"}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	time.Sleep(300 * time.Millisecond)
	w = request(t, h, "GET", "/api/tasks", "")
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("succeeded")) {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestTaskScopeValidationAndStoryResultWriteback(t *testing.T) {
	s, done := testServer(t)
	defer done()
	h := testRouter(s)
	missing := request(t, h, http.MethodPost, "/api/tasks", `{"kind":"story","prompt":"test"}`)
	if missing.Code != http.StatusBadRequest || !strings.Contains(missing.Body.String(), "missing_task_scope") {
		t.Fatalf("missing scope: %d %s", missing.Code, missing.Body.String())
	}
	wrong := request(t, h, http.MethodPost, "/api/tasks", `{"project_id":"test-project","draft_id":"wrong","kind":"story","prompt":"test"}`)
	if wrong.Code != http.StatusBadRequest || !strings.Contains(wrong.Body.String(), "draft_not_found") {
		t.Fatalf("wrong draft: %d %s", wrong.Code, wrong.Body.String())
	}
	created := request(t, h, http.MethodPost, "/api/tasks", scopedTaskJSON(`"kind":"story","prompt":"generated body"`))
	if created.Code != http.StatusAccepted {
		t.Fatal(created.Code, created.Body.String())
	}
	var item task.Task
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		current, _ := s.TaskService.Store.Get(context.Background(), item.ID)
		if current.Status == task.StatusSucceeded {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	p, err := s.Projects.Repo.Get(context.Background(), "test-project")
	if err != nil || p.Drafts[0].Story.Body != "generated body" || len(p.Drafts[0].Outputs) != 1 {
		t.Fatalf("story result not applied: %#v, %v", p, err)
	}
	saved := request(t, h, http.MethodPost, "/api/projects/test-project/drafts", `{"id":"test-draft","name":"Draft","story":{"body":"user edit","scenes":[],"updated_at":"2026-09-22T00:00:00Z"}}`)
	if saved.Code != http.StatusOK {
		t.Fatal(saved.Code, saved.Body.String())
	}
	p, err = s.Projects.Repo.Get(context.Background(), "test-project")
	if err != nil || p.Drafts[0].Story.Body != "user edit" || len(p.Drafts[0].Outputs) != 1 {
		t.Fatalf("draft save lost generated outputs: %#v, %v", p, err)
	}
	filtered := request(t, h, http.MethodGet, "/api/tasks?project_id=test-project&draft_id=test-draft", "")
	if filtered.Code != http.StatusOK || !strings.Contains(filtered.Body.String(), item.ID) {
		t.Fatalf("scoped query: %d %s", filtered.Code, filtered.Body.String())
	}
}

func TestAuthentication(t *testing.T) {
	s, done := testServer(t)
	defer done()
	s.AuthToken = "secret"
	w := request(t, testRouter(s), "GET", "/api/tasks", "")
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}

func TestRoutesRestrictMethodsAndRejectUnknownActions(t *testing.T) {
	s, done := testServer(t)
	defer done()
	h := testRouter(s)
	for _, route := range []struct {
		method, path string
		status       int
	}{
		{"POST", "/health", 405},
		{"PATCH", "/api/providers/test", 405},
		{"GET", "/api/projects/test/drafts", 405},
		{"GET", "/api/tasks/test/cancel", 405},
		{"POST", "/ws", 405},
		{"GET", "/api/tasks/test/events", 404},
		{"GET", "/api/tasks/test/unknown", 404},
		{"GET", "/api/providers/test/test/unknown", 404},
	} {
		if got := request(t, h, route.method, route.path, "").Code; got != route.status {
			t.Errorf("%s %s: got %d, want %d", route.method, route.path, got, route.status)
		}
	}
}

func TestShutdownEndpoint(t *testing.T) {
	s, done := testServer(t)
	defer done()
	requested := make(chan struct{}, 1)
	s.Shutdown = func() { requested <- struct{}{} }
	w := request(t, testRouter(s), http.MethodPost, "/api/system/shutdown", "")
	if w.Code != http.StatusAccepted {
		t.Fatal(w.Code, w.Body.String())
	}
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["status"] != "shutting_down" {
		t.Fatalf("unexpected response: %v", response)
	}
	select {
	case <-requested:
	case <-time.After(time.Second):
		t.Fatal("shutdown callback was not called")
	}
}

func TestShutdownEndpointUnavailable(t *testing.T) {
	s, done := testServer(t)
	defer done()
	w := request(t, testRouter(s), http.MethodPost, "/api/system/shutdown", "")
	if w.Code != http.StatusNotImplemented {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestTaskEventsUpgradeThroughMiddleware(t *testing.T) {
	s, done := testServer(t)
	defer done()
	created := request(t, testRouter(s), http.MethodPost, "/api/tasks", scopedTaskJSON(`"kind":"video","prompt":"测试视频","source_image_url":"https://example.com/frame.png"`))
	if created.Code != http.StatusAccepted {
		t.Fatal(created.Code, created.Body.String())
	}
	var item struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(testRouter(s))
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, response, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		if response != nil {
			t.Fatalf("websocket status %d: %v", response.StatusCode, err)
		}
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	ws.Connections.Broadcast(context.Background(), task.Event{TaskID: item.ID, Status: task.StatusRunning})
	var event map[string]any
	if err = conn.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}
	if event["task_id"] != item.ID {
		t.Fatal(event)
	}
}

func TestTaskUpdatesBroadcastWithoutClientSession(t *testing.T) {
	s, done := testServer(t)
	defer done()
	s.TaskService.Enqueuer = nil
	server := httptest.NewServer(testRouter(s))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	created := request(t, testRouter(s), http.MethodPost, "/api/tasks", scopedTaskJSON(`"kind":"story","prompt":"broadcast"`))
	if created.Code != http.StatusAccepted {
		t.Fatal(created.Code, created.Body.String())
	}
	var item task.Task
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	var event task.Event
	if err := conn.ReadJSON(&event); err != nil || event.TaskID != item.ID || event.Status != task.StatusQueued {
		t.Fatal(event, err)
	}
	stored, err := s.TaskService.Store.Get(context.Background(), item.ID)
	if err != nil || stored.Status != task.StatusQueued {
		t.Fatal(stored, err)
	}
	processor := testProcessor(s)
	processor.Execute(context.Background(), item)
	for event.Status != task.StatusSucceeded {
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
	}
	if event.ProjectID != item.ProjectID || event.DraftID != item.DraftID {
		t.Fatalf("completion event lost task scope: %+v", event)
	}
	stored, err = s.TaskService.Store.Get(context.Background(), item.ID)
	if err != nil || stored.Status != event.Status || stored.Progress != event.Progress {
		t.Fatal(stored, event, err)
	}
}

func TestTaskCreationPerformanceAndMetrics(t *testing.T) {
	s, done := testServer(t)
	defer done()
	handler := testRouter(s)
	for i := 0; i < 20; i++ {
		started := time.Now()
		response := request(t, handler, http.MethodPost, "/api/tasks", scopedTaskJSON(`"kind":"story","prompt":"测试视频"`))
		if response.Code != http.StatusAccepted {
			t.Fatal(response.Code, response.Body.String())
		}
		if elapsed := time.Since(started); elapsed >= time.Second {
			t.Fatalf("task creation exceeded one second: %s", elapsed)
		}
	}
	metrics := request(t, handler, http.MethodGet, "/metrics", "")
	if metrics.Code != http.StatusOK || !strings.Contains(metrics.Body.String(), "frameflow_queue_depth") || !strings.Contains(metrics.Body.String(), "frameflow_tasks") {
		t.Fatal(metrics.Code, metrics.Body.String())
	}
}

type testServices struct {
	Worker               *queue.Worker
	TaskService          application.TaskService
	Projects             application.ProjectService
	Providers            application.ProviderService
	Materials            application.MaterialService
	AuthToken, UploadDir string
	Vault                provider.CredentialVault
	RateLimit            int
	Shutdown             func()
}

func testRouter(s testServices) *gin.Engine {
	engine := gin.Default()
	UseMiddleware(engine, s.AuthToken, s.RateLimit)
	s.Providers.Vault = s.Vault
	s.Materials.UploadDir = s.UploadDir
	RegisterRoutes(engine, s.TaskService, s.Projects, s.Providers, s.Materials, application.SystemService{Tasks: s.TaskService, Projects: s.Projects, Providers: s.Providers, Materials: s.Materials, Shutdown: s.Shutdown, QueueDepth: s.Worker.Depth})
	engine.GET("/ws", ws.Handler)
	return engine
}

func TestTaskPersistsWithoutSessionAndProcessorCompletes(t *testing.T) {
	services, done := testServer(t)
	defer done()
	services.TaskService.Enqueuer = nil
	response := request(t, testRouter(services), "POST", "/api/tasks", scopedTaskJSON(`"kind":"story","prompt":"session test"`))
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	var item task.Task
	if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	current, err := services.TaskService.Store.Get(context.Background(), item.ID)
	if err != nil || current.Status != task.StatusQueued {
		t.Fatal(current, err)
	}
	processor := testProcessor(services)
	processor.Execute(context.Background(), current)
	current, err = services.TaskService.Store.Get(context.Background(), item.ID)
	if err != nil || current.Status != task.StatusSucceeded {
		t.Fatal(current, err)
	}
}

func scopedTaskJSON(fields string) string {
	return `{"project_id":"test-project","draft_id":"test-draft",` + fields + `}`
}
