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
	"github.com/fengxiaozi-liu/FrameFlow/internal/transport/response"
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
func TestServicePassesContextAndBindsJSON(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "request-value")
	repo := &recordingProjects{}
	service := application.ProjectService{Repo: repo}
	router := gin.New()
	router.Use(response.Middleware())
	router.POST("/", service.Create)
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"demo"}`)).WithContext(ctx)
	out := httptest.NewRecorder()
	router.ServeHTTP(out, req)
	if repo.ctx != ctx || repo.calls != 1 || out.Code != 201 || !strings.Contains(out.Body.String(), `"name":"demo"`) {
		t.Fatalf("%d %s", out.Code, out.Body.String())
	}
	out = request(t, router, "POST", "/", `{"unknown":"value"}`)
	if repo.calls != 1 || out.Code != 400 {
		t.Fatalf("invalid body reached repository: %d", out.Code)
	}
}

func TestResponseErrorsAndNoContent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"cancel", context.Canceled, 408, "request_cancelled"},
		{"timeout", context.DeadlineExceeded, 504, "deadline_exceeded"},
		{"missing", fault.ErrNotFound, 404, "not_found"},
		{"storage", errors.New("secret database path"), 500, "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(response.Middleware())
			router.GET("/", func(c *gin.Context) { response.Set(c, 200, nil, tc.err) })
			out := request(t, router, "GET", "/", "")
			if out.Code != tc.status || !strings.Contains(out.Body.String(), tc.code) || strings.Contains(out.Body.String(), "secret database path") {
				t.Fatalf("%d %s", out.Code, out.Body.String())
			}
		})
	}
	router := gin.New()
	router.Use(response.Middleware())
	router.DELETE("/:id", func(c *gin.Context) { response.Set(c, 204, nil, nil) })
	out := request(t, router, "DELETE", "/task", "")
	if out.Code != 204 || out.Body.Len() != 0 {
		t.Fatalf("%d %s", out.Code, out.Body.String())
	}
	router.GET("/raw", func(c *gin.Context) { c.String(200, "raw") })
	out = request(t, router, "GET", "/raw", "")
	if out.Body.String() != "raw" {
		t.Fatal("middleware rewrote raw response")
	}
}

func TestCancelledRequestDoesNotInvokeApplication(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	router := gin.New()
	router.Use(response.Middleware())
	router.GET("/", func(c *gin.Context) { t.Error("cancelled request reached service") })
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
	if out.Code != 504 {
		t.Fatal(out.Code)
	}
}
