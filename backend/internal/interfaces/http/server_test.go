package http

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	domainprocessor "github.com/fengxiaozi-liu/FrameFlow/internal/domain/processor"
	providerinfra "github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testServer(t *testing.T) (Server, func()) {
	db, e := sqlite.Open(filepath.Join(t.TempDir(), "api.db"))
	if e != nil {
		t.Fatal(e)
	}
	worker := queue.NewWorker(db)
	providers := application.ProviderService{Repo: sqlite.NewProviderRepository(db)}
	processor := domainprocessor.New(providers.Repo, providerinfra.NewRegistry(), worker)
	ctx, cancel := context.WithCancel(context.Background())
	go processor.Start(ctx)
	tasks := application.TaskService{Store: db, Enqueuer: processor, Providers: providers}
	return Server{Store: db, Worker: worker, TaskService: tasks, Projects: application.ProjectService{Repo: sqlite.NewProjectRepository(db)}, Providers: providers, Materials: application.MaterialService{Repo: sqlite.NewMaterialRepository(db)}}, func() { cancel(); _ = db.Close() }
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
	h := s.Routes()
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
	w := request(t, s.Routes(), "GET", "/api/tasks", "")
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}

func TestShutdownEndpoint(t *testing.T) {
	s, done := testServer(t)
	defer done()
	requested := make(chan struct{}, 1)
	s.Shutdown = func() { requested <- struct{}{} }
	w := request(t, s.Routes(), http.MethodPost, "/api/system/shutdown", "")
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
	w := request(t, s.Routes(), http.MethodPost, "/api/system/shutdown", "")
	if w.Code != http.StatusNotImplemented {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestTaskEventsUpgradeThroughMiddleware(t *testing.T) {
	s, done := testServer(t)
	defer done()
	created := request(t, s.Routes(), http.MethodPost, "/api/tasks", `{"kind":"video","prompt":"测试视频"}`)
	if created.Code != http.StatusAccepted {
		t.Fatal(created.Code, created.Body.String())
	}
	var item struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(s.Routes())
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/tasks/" + item.ID + "/events"
	conn, response, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		if response != nil {
			t.Fatalf("websocket status %d: %v", response.StatusCode, err)
		}
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var event map[string]any
	if err = conn.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}
}

func TestTaskCreationPerformanceAndMetrics(t *testing.T) {
	s, done := testServer(t)
	defer done()
	handler := s.Routes()
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
