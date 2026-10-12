package http

import (
	"context"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"sync"
	"testing"
)

func TestRunPersistsAndDoesNotDuplicateChildren(t *testing.T) {
	s, done := testServer(t)
	defer done()
	ctx := context.Background()
	if err := s.Providers.Catalog.Models.SaveModel(ctx, provider.Model{ID: "vid", ConnectionID: "test-connection", RemoteID: "wan2.7-i2v", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Video}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Providers.Catalog.Models.SaveModel(ctx, provider.Model{ID: "im", ConnectionID: "test-connection", RemoteID: "qwen-image-2.0-pro", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Image}}); err != nil {
		t.Fatal(err)
	}
	_, err := s.Projects.Repo.(project.AtomicRepository).Update(ctx, "test-project", func(p *project.Project) error {
		p.Drafts[0].Story.Scenes = []story.Scene{{ID: "s", VisualPrompt: "cafe", DurationSeconds: 4}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	router := testRouter(s)
	path := "/api/projects/test-project/drafts/test-draft/scenes/s/generation-runs"
	bad := request(t, router, "POST", path, `{"expected_version":0,"idempotency_key":"bad","model_id":"im","video_model_id":"missing","path":"reference_image_then_video"}`)
	if bad.Code != 400 {
		t.Fatal(bad.Code, bad.Body.String())
	}
	empty, _ := s.TaskService.Store.List(ctx)
	if len(empty) != 0 {
		t.Fatal("invalid video model submitted a paid image task")
	}
	var wg sync.WaitGroup
	codes := make(chan int, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := request(t, router, "POST", path, `{"expected_version":0,"idempotency_key":"run","model_id":"im","video_model_id":"vid","path":"reference_image_then_video"}`)
			codes <- response.Code
		}()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 202 {
			t.Fatal("concurrent submit failed", code)
		}
	}
	r := request(t, router, "POST", path, `{"expected_version":0,"idempotency_key":"run","model_id":"im","video_model_id":"vid","path":"reference_image_then_video"}`)
	if r.Code != 202 {
		t.Fatal(r.Code, r.Body.String())
	}
	var run task.GenerationRun
	_ = json.Unmarshal(r.Body.Bytes(), &run)
	for i := 0; i < 3; i++ {
		r = request(t, router, "GET", path+"/"+run.ID, "")
		if r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	all, err := s.TaskService.Store.List(ctx)
	if err != nil || len(all) != 1 {
		t.Fatal("duplicate children", len(all), err)
	}
	r = request(t, router, "POST", path+"/"+run.ID+"/cancel", `{}`)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	r = request(t, router, "GET", path+"/"+run.ID, "")
	_ = json.Unmarshal(r.Body.Bytes(), &run)
	if run.State != "cancelled" {
		t.Fatal(run)
	}
	p, _ := s.Projects.Repo.Get(ctx, "test-project")
	if len(p.Drafts[0].GenerationRuns) != 1 {
		t.Fatal("concurrent request duplicated run")
	}
	r = request(t, router, "POST", "/api/tasks/"+run.ID+"-0/retry", `{}`)
	if r.Code != 409 {
		t.Fatal("cancelled run restarted", r.Code, r.Body.String())
	}
}
