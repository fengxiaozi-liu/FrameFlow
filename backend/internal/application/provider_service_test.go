package application

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

type providerRepo struct{ m map[string]provider.Config }

func (r *providerRepo) Save(ctx context.Context, c provider.Config) error {
	r.m[c.Code] = c
	return nil
}

func (r *providerRepo) Get(ctx context.Context, id string) (provider.Config, error) {
	c, o := r.m[id]
	if !o {
		return c, fault.ErrNotFound
	}
	return c, nil
}

func (r *providerRepo) List(ctx context.Context, k provider.Capability) ([]provider.Config, error) {
	o := []provider.Config{}
	for _, c := range r.m {
		if c.Capability == k {
			o = append(o, c)
		}
	}
	return o, nil
}

func (r *providerRepo) Delete(ctx context.Context, id string) error {
	delete(r.m, id)
	return nil
}

func TestProviderService(t *testing.T) {
	r := &providerRepo{m: map[string]provider.Config{}}
	s := ProviderService{Repo: r}
	router := gin.New()
	router.POST("/providers", s.Save)
	router.GET("/providers", s.List)
	router.POST("/providers/:id/enabled", s.SetEnabled)
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/providers", `{"code":"video-a","name":"Video A","capability":"video"}`, 201},
		{"GET", "/providers?capability=video", "", 200},
		{"POST", "/providers/video-a/enabled", `{"enabled":true}`, 200},
	} {
		out := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(out, req)
		if out.Code != tc.status {
			t.Fatal(out.Code, out.Body.String())
		}
	}
	if !r.m["video-a"].Enabled {
		t.Fatal("provider was not enabled")
	}
}
