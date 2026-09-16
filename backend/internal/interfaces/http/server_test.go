package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	domainprocessor "github.com/fengxiaozi-liu/FrameFlow/internal/domain/processor"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	providerinfra "github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	ws "github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/websocket"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testServer(t *testing.T) (testServices, func()) {
	db, e := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "api.db"))
	if e != nil {
		t.Fatal(e)
	}
	worker := queue.NewWorker()
	providers := application.ProviderService{Repo: sqlite.NewProviderRepository(db)}
	processor := domainprocessor.New(providers.Repo, providerinfra.NewRegistry(), worker, db)
	processor.Events = db
	processor.SendEvent = ws.Send
	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); processor.Start(ctx) }()
	tasks := application.TaskService{Store: db, Enqueuer: processor, Events: db, SendEvent: ws.Send, Providers: providers}
	return testServices{Worker: worker, TaskService: tasks, Projects: application.ProjectService{Repo: sqlite.NewProjectRepository(db)}, Providers: providers, Materials: application.MaterialService{Repo: sqlite.NewMaterialRepository(db)}}, func() { cancel(); <-workerDone; ws.CloseAll(); _ = db.Close() }
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
	w = request(t, h, "POST", "/api/tasks", `{"kind":"video","prompt":"测试视频"}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	time.Sleep(300 * time.Millisecond)
	w = request(t, h, "GET", "/api/tasks", "")
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("succeeded")) {
		t.Fatal(w.Code, w.Body.String())
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
	created := request(t, testRouter(s), http.MethodPost, "/api/tasks", `{"kind":"video","prompt":"测试视频"}`)
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
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?session_id=" + item.ID
	conn, response, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		if response != nil {
			t.Fatalf("websocket status %d: %v", response.StatusCode, err)
		}
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_ = ws.Send(context.Background(), item.ID, task.Event{TaskID: item.ID, Status: task.StatusRunning})
	var event map[string]any
	if err = conn.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}
}

func TestTaskCreationPerformanceAndMetrics(t *testing.T) {
	s, done := testServer(t)
	defer done()
	handler := testRouter(s)
	for i := 0; i < 20; i++ {
		started := time.Now()
		response := request(t, handler, http.MethodPost, "/api/tasks", `{"kind":"video","prompt":"测试视频"}`)
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

func TestSessionPersistsAndDeliveryFailureDoesNotFailTask(t *testing.T) {
	services, done := testServer(t)
	defer done()
	services.TaskService.Enqueuer = nil
	calls := 0
	send := func(ctx context.Context, sessionID string, event task.Event) error {
		if sessionID != "page-session" {
			t.Fatalf("unexpected session %s", sessionID)
		}
		events, err := services.TaskService.Events.ListEvents(ctx, event.TaskID, 0)
		if err != nil || len(events) == 0 || events[len(events)-1].Sequence != event.Sequence {
			t.Fatal("notification sent before persistence", err)
		}
		calls++
		return errors.New("connection closed")
	}
	services.TaskService.SendEvent = send
	response := request(t, testRouter(services), "POST", "/api/tasks", `{"prompt":"session test","session_id":"page-session"}`)
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	var item task.Task
	if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	current, err := services.TaskService.Store.Get(context.Background(), item.ID)
	if err != nil || current.SessionID != "page-session" {
		t.Fatal(current, err)
	}
	processor := domainprocessor.New(services.Providers.Repo, providerinfra.NewRegistry(), queue.NewWorker(), services.TaskService.Store)
	processor.SendEvent = send
	processor.Execute(context.Background(), current)
	current, err = services.TaskService.Store.Get(context.Background(), item.ID)
	if err != nil || current.Status != task.StatusSucceeded || calls < 2 {
		t.Fatal(current, calls, err)
	}
}
