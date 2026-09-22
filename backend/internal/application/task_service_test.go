package application

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type taskTestProjects struct{ value project.Project }

func (r taskTestProjects) Save(context.Context, project.Project) error          { return nil }
func (r taskTestProjects) Get(context.Context, string) (project.Project, error) { return r.value, nil }
func (r taskTestProjects) List(context.Context) ([]project.Project, error) {
	return []project.Project{r.value}, nil
}
func (r taskTestProjects) Delete(context.Context, string) error { return nil }

func TestCancelledSubmissionDoesNotLeaveUnscheduledQueuedTask(t *testing.T) {
	worker := queue.NewWorker()
	for i := 0; i < 32; i++ {
		if err := worker.Enqueue(context.Background(), task.Task{}); err != nil {
			t.Fatal(err)
		}
	}
	store := queue.NewStore()
	p, _ := project.New("project", "test", time.Now())
	_ = p.AddDraft(project.Draft{ID: "draft"})
	service := TaskService{Store: store, Enqueuer: worker, Projects: taskTestProjects{value: p}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	router := gin.New()
	router.POST("/tasks", service.Create)
	out := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/tasks", strings.NewReader(`{"project_id":"project","draft_id":"draft","kind":"video","prompt":"test"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(out, req)
	if out.Code != 504 {
		t.Fatal(out.Code, out.Body.String())
	}
	items, err := store.List(context.Background())
	if err != nil || len(items) != 1 || items[0].Status != task.StatusCancelled {
		t.Fatal(items, err)
	}
}

func TestDownloadServesSavedMediaAndKeepsJSONResult(t *testing.T) {
	for _, test := range []struct {
		kind task.Kind
		ext  string
	}{
		{task.KindImage, ".png"},
		{task.KindVideo, ".mp4"},
	} {
		t.Run(string(test.kind), func(t *testing.T) {
			dir := t.TempDir()
			store := queue.NewStore()
			item := task.New("20260914160358.943272300", test.kind, task.Input{Prompt: "test"}, time.Now())
			item.Succeed(time.Now())
			name := "generated-" + item.ID + test.ext
			item.ResultURL = "/media/" + name
			if err := store.Save(context.Background(), item); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte("media-data"), 0600); err != nil {
				t.Fatal(err)
			}
			service := TaskService{Store: store, UploadDir: dir}
			router := gin.New()
			router.GET("/tasks/:id/result", service.Result)
			router.GET("/tasks/:id/download", service.Download)
			request := func(path string) *httptest.ResponseRecorder {
				out := httptest.NewRecorder()
				router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, path, nil))
				return out
			}
			result := request("/tasks/" + item.ID + "/result")
			if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), item.ResultURL) {
				t.Fatalf("JSON result changed: %d %s", result.Code, result.Body.String())
			}
			download := request("/tasks/" + item.ID + "/download")
			if download.Code != http.StatusOK || download.Body.String() != "media-data" || !strings.Contains(download.Header().Get("Content-Disposition"), "frameflow-"+item.ID+test.ext) {
				t.Fatalf("media download: %d %q %q", download.Code, download.Body.String(), download.Header().Get("Content-Disposition"))
			}
			contentType := "image/png"
			if test.kind == task.KindVideo {
				contentType = "video/mp4"
			}
			if got := download.Header().Get("Content-Type"); got != contentType {
				t.Fatalf("wrong media content type: %q", got)
			}
			item.ResultURL = ""
			if err := store.Save(context.Background(), item); err != nil {
				t.Fatal(err)
			}
			if got := request("/tasks/" + item.ID + "/download"); got.Code != http.StatusConflict {
				t.Fatalf("missing media returned %d", got.Code)
			}
		})
	}
}
