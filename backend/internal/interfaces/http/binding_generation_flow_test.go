package http

import (
	"context"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"testing"
)

func TestBindingGenerationIdempotencyAndSnapshot(t *testing.T) {
	services, done := testServer(t)
	defer done()
	ctx := context.Background()
	if err := services.Providers.Catalog.Models.SaveModel(ctx, provider.Model{ID: "image", ConnectionID: "test-connection", RemoteID: "qwen-image-2.0-pro", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Image}}); err != nil {
		t.Fatal(err)
	}
	_, err := services.Projects.Repo.(project.AtomicRepository).Update(ctx, "test-project", func(p *project.Project) error {
		p.Drafts[0].Story.Scenes = []story.Scene{{ID: "s", VisualPrompt: "A quiet cafe", DurationSeconds: 4}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	router := testRouter(services)
	path := "/api/projects/test-project/drafts/test-draft/scenes/s/image-tasks"
	body := `{"expected_version":0,"idempotency_key":"same","model_id":"image","path":"reference_image_then_video","aspect_ratio":"16:9"}`
	first := request(t, router, "POST", path, body)
	if first.Code != 202 {
		t.Fatal(first.Code, first.Body.String())
	}
	var item task.Task
	_ = json.Unmarshal(first.Body.Bytes(), &item)
	if item.Snapshot == nil || item.Snapshot.Prompt != "A quiet cafe" {
		t.Fatal(item)
	}
	repeated := request(t, router, "POST", path, body)
	if repeated.Code != 202 {
		t.Fatal(repeated.Body.String())
	}
	var again task.Task
	_ = json.Unmarshal(repeated.Body.Bytes(), &again)
	if again.ID != item.ID {
		t.Fatal("duplicate task")
	}
	changed := request(t, router, "POST", path, `{"expected_version":0,"idempotency_key":"same","model_id":"image","path":"reference_image_then_video","aspect_ratio":"9:16"}`)
	if changed.Code != 409 {
		t.Fatal(changed.Code, changed.Body.String())
	}
	item.Snapshot.Prompt = "overwrite"
	if err := services.TaskService.Store.Save(ctx, item); err == nil {
		t.Fatal("snapshot rewritten")
	}
}
