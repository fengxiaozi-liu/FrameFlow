package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/gin-gonic/gin"
)

type recordingProjects struct {
	project.Repository
	ctx   context.Context
	calls int
}

func (r *recordingProjects) Save(ctx context.Context, p project.Project) error {
	r.ctx = ctx
	r.calls++
	return nil
}
func TestProjectServiceBindsAndWritesJSON(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "request-value")
	repo := &recordingProjects{}
	service := application.ProjectService{Repo: repo}
	router := gin.New()
	router.Use(limitJSONBody())
	router.POST("/", service.Create)
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"demo"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	out := httptest.NewRecorder()
	router.ServeHTTP(out, req)
	if repo.ctx != ctx || repo.calls != 1 || out.Code != 201 || !strings.Contains(out.Body.String(), `"name":"demo"`) {
		t.Fatalf("%d %s", out.Code, out.Body.String())
	}
	out = request(t, router, "POST", "/", `{"unknown":"value"}`)
	if repo.calls != 1 || out.Code != 400 {
		t.Fatalf("invalid body reached repository: %d", out.Code)
	}
	out = request(t, router, "POST", "/", `{"name":`)
	if repo.calls != 1 || out.Code != 400 {
		t.Fatalf("malformed body reached repository: %d", out.Code)
	}
	out = request(t, router, "POST", "/", `{"name":"valid","unknown":"value"}`)
	if repo.calls != 2 || out.Code != 201 {
		t.Fatalf("Gin binding rejected a valid project: %d %s", out.Code, out.Body.String())
	}
	out = request(t, router, "POST", "/", `{"name":"`+strings.Repeat("x", 1<<20)+`"}`)
	if repo.calls != 2 || out.Code != http.StatusBadRequest {
		t.Fatalf("oversized JSON reached repository: %d", out.Code)
	}
}

type failingProjects struct {
	project.Repository
	err error
}

func (r failingProjects) Save(context.Context, project.Project) error {
	return r.err
}

func (r failingProjects) Get(context.Context, string) (project.Project, error) {
	return project.Project{}, r.err
}

func (r failingProjects) List(context.Context) ([]project.Project, error) {
	return nil, r.err
}

func TestProjectServiceErrorResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "not found", err: fault.ErrNotFound, status: 404, code: "not_found"},
		{name: "storage", err: errors.New("secret database path"), status: 500, code: "internal_error"},
		{name: "cancel", err: context.Canceled, status: 408, code: "request_cancelled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := application.ProjectService{Repo: failingProjects{err: tc.err}}
			router := gin.New()
			router.POST("/projects", service.Create)
			router.GET("/projects", service.List)
			router.GET("/projects/:id", service.Get)
			router.POST("/projects/:id/drafts", service.SaveDraft)
			for _, call := range []struct {
				method, path, body string
			}{
				{http.MethodPost, "/projects", `{"name":"demo"}`},
				{http.MethodGet, "/projects", ""},
				{http.MethodGet, "/projects/1", ""},
				{http.MethodPost, "/projects/1/drafts", `{"name":"draft"}`},
			} {
				out := request(t, router, call.method, call.path, call.body)
				if out.Code != tc.status || !strings.Contains(out.Body.String(), tc.code) || strings.Contains(out.Body.String(), "secret database path") {
					t.Fatalf("%s %s: %d %s", call.method, call.path, out.Code, out.Body.String())
				}
			}
		})
	}
}

func TestNoContentAndRawResponse(t *testing.T) {
	router := gin.New()
	router.DELETE("/:id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	out := request(t, router, "DELETE", "/task", "")
	if out.Code != 204 || out.Body.Len() != 0 {
		t.Fatalf("%d %s", out.Code, out.Body.String())
	}
	router.GET("/raw", func(c *gin.Context) { c.String(200, "raw") })
	out = request(t, router, "GET", "/raw", "")
	if out.Body.String() != "raw" {
		t.Fatal("raw response changed")
	}
}

func TestCancelledRequestDoesNotInvokeApplication(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	router := gin.New()
	UseMiddleware(router, "", 0)
	router.GET("/", func(c *gin.Context) { t.Error("cancelled request reached service") })
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
	if out.Code != 504 {
		t.Fatal(out.Code)
	}
}
