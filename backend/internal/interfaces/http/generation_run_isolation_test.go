package http

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"os"
	"path/filepath"
	"testing"
)

func TestRunSelectionIsolatedAndFrozen(t *testing.T) {
	s, done := testServer(t)
	defer done()
	ctx := context.Background()
	s.TaskService.UploadDir = t.TempDir()
	// This test publishes fixture results itself. Keep submitted jobs in a
	// separate queue so the background fake generator cannot overwrite them.
	s.TaskService.Enqueuer = queue.NewWorker()
	for _, m := range []provider.Model{{ID: "im", ConnectionID: "test-connection", RemoteID: "qwen-image-2.0-pro", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Image}}, {ID: "vid", ConnectionID: "test-connection", RemoteID: "wan2.7-i2v", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Video}}} {
		if e := s.Providers.Catalog.Models.SaveModel(ctx, m); e != nil {
			t.Fatal(e)
		}
	}
	_, e := s.Projects.Repo.(project.AtomicRepository).Update(ctx, "test-project", func(p *project.Project) error {
		p.Drafts[0].Story.Scenes = []story.Scene{{ID: "s", VisualPrompt: "original cafe", DurationSeconds: 4}, {ID: "other", VisualPrompt: "other"}}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	router := testRouter(s)
	path := "/api/projects/test-project/drafts/test-draft/scenes/s/generation-runs"
	body := func(key string) string {
		return fmt.Sprintf(`{"expected_version":0,"idempotency_key":%q,"model_id":"im","video_model_id":"vid","path":"reference_image_then_video"}`, key)
	}
	r := request(t, router, "POST", path, body("one"))
	if r.Code != 202 {
		t.Fatal(r.Body.String())
	}
	var a task.GenerationRun
	_ = json.Unmarshal(r.Body.Bytes(), &a)
	r = request(t, router, "POST", path, body("one"))
	var duplicate task.GenerationRun
	_ = json.Unmarshal(r.Body.Bytes(), &duplicate)
	if duplicate.ID != a.ID {
		t.Fatal("same click duplicated")
	}
	r = request(t, router, "POST", path, body("two"))
	var b task.GenerationRun
	_ = json.Unmarshal(r.Body.Bytes(), &b)
	r = request(t, router, "GET", "/api/projects/test-project/drafts/test-draft/scenes/other/generation-runs/"+a.ID, "")
	if r.Code == 200 {
		t.Fatal("cross shot run exposed")
	}
	if err := os.WriteFile(filepath.Join(s.TaskService.UploadDir, "frame.png"), []byte("frame bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.TaskService.Materials.Save(ctx, material.Asset{ID: "frame", Kind: material.Scene, URL: "/media/frame.png"}); err != nil {
		t.Fatal(err)
	}
	for _, run := range []task.GenerationRun{a, b} {
		item, err := s.TaskService.Store.Get(ctx, run.ID+"-0")
		if err != nil {
			t.Fatal(err)
		}
		item.Status = task.StatusSucceeded
		if err = s.TaskService.Store.Save(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	_, e = s.Projects.Repo.(project.AtomicRepository).Update(ctx, "test-project", func(p *project.Project) error {
		d := &p.Drafts[0]
		d.Story.Scenes[0].VisualPrompt = "changed cafe"
		d.MediaTakes = []project.MediaTake{{ID: "a-image", Kind: "image", SceneID: "s", TaskID: a.ID + "-0", MaterialID: "frame"}, {ID: "b-image", Kind: "image", SceneID: "s", TaskID: b.ID + "-0", MaterialID: "frame"}}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	r = request(t, router, "PUT", path+"/"+a.ID+"/selected-media", `{"expected_version":0,"take_id":"b-image"}`)
	if r.Code == 200 {
		t.Fatal("cross run selection accepted")
	}
	r = request(t, router, "PUT", path+"/"+a.ID+"/selected-media", `{"expected_version":0,"take_id":"a-image"}`)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	_ = json.Unmarshal(r.Body.Bytes(), &a)
	if len(a.Plan) != 2 || a.Plan[1].Prompt != "original cafe" || a.Selection.ImageTakeID != "a-image" {
		t.Fatal("video did not use frozen input", a)
	}
	r = request(t, router, "GET", path, "")
	var list struct {
		Runs []task.GenerationRun `json:"runs"`
	}
	_ = json.Unmarshal(r.Body.Bytes(), &list)
	if len(list.Runs) != 2 || list.Runs[1].Selection.ImageTakeID != "" {
		t.Fatal("wrong run count or shared selection")
	}
	videoID := a.ID + "-1"
	video, err := s.TaskService.Store.Get(ctx, videoID)
	if err != nil {
		t.Fatal(err)
	}
	video.Status = task.StatusSucceeded
	if err = s.TaskService.Store.Save(ctx, video); err != nil {
		t.Fatal(err)
	}
	_, err = s.Projects.Repo.(project.AtomicRepository).Update(ctx, "test-project", func(p *project.Project) error {
		p.Drafts[0].VideoVersions = []project.VideoVersion{{ID: "version-" + videoID, SceneID: "s", TaskID: videoID, LocalMediaPath: "/media/video.mp4", DurationSeconds: 4}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	r = request(t, router, "PUT", path+"/"+b.ID+"/selected-media", fmt.Sprintf(`{"expected_version":1,"video_version_id":%q}`, "version-"+videoID))
	if r.Code == 200 {
		t.Fatal("cross run video selection accepted")
	}
	r = request(t, router, "PUT", path+"/"+a.ID+"/selected-media", fmt.Sprintf(`{"expected_version":1,"video_version_id":%q}`, "version-"+videoID))
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	_ = json.Unmarshal(r.Body.Bytes(), &a)
	if a.State != "awaiting_composition" || a.Selection.VideoVersionID != "version-"+videoID {
		t.Fatal("video selection not published", a)
	}

}

func TestShotSettingsAndLocalReferencesPersist(t *testing.T) {
	s, done := testServer(t)
	defer done()
	ctx := context.Background()
	for _, m := range []provider.Model{{ID: "im", ConnectionID: "test-connection", RemoteID: "qwen-image-2.0-pro", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Image}}, {ID: "vid", ConnectionID: "test-connection", RemoteID: "wan2.7-i2v", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Video}}} {
		if err := s.Providers.Catalog.Models.SaveModel(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.TaskService.Materials.Save(ctx, material.Asset{ID: "local", Name: "local image", Kind: material.Character, URL: "/media/local.png"}); err != nil {
		t.Fatal(err)
	}
	_, err := s.Projects.Repo.(project.AtomicRepository).Update(ctx, "test-project", func(p *project.Project) error {
		p.EntityCatalog.Entities = []project.EntityVersion{{ID: "actor", Kind: "character", Name: "林夏", Revision: 1, References: []project.ReferenceImage{}}}
		p.Drafts[0].Story.Scenes = []story.Scene{{ID: "s", Title: "Original", DurationSeconds: 4}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	router := testRouter(s)
	path := "/api/projects/test-project/drafts/test-draft/scenes/s/context"
	body := `{"expected_version":0,"title":"雨夜进店","narration":"雨声渐弱","entity_bindings":[{"id":"actor-binding","entity_id":"actor","entity_revision":1,"role":"cast","reference_material_ids":["local"]}],"prompt_document":{"schema_version":1,"nodes":[{"type":"text","text":"雨夜进店"}]},"utterances":[],"generation_settings":{"image_model_id":"im","video_model_id":"vid","aspect_ratio":"9:16","resolution":"1080P","duration_seconds":6}}`
	r := request(t, router, "PUT", path, body)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	p, err := s.Projects.Repo.Get(ctx, "test-project")
	if err != nil {
		t.Fatal(err)
	}
	scene := p.Drafts[0].Story.Scenes[0]
	if scene.Title != "雨夜进店" || scene.Narration != "雨声渐弱" || scene.DurationSeconds != 6 || scene.GenerationSettings.VideoModelID != "vid" || len(*scene.EntityBindings[0].ReferenceMaterialIDs) != 1 || len(p.EntityCatalog.Entities[0].References) != 0 {
		t.Fatal("configuration or entity scope changed", p)
	}
	r = request(t, router, "DELETE", "/api/materials/local", "")
	if r.Code != 409 {
		t.Fatal("bound local image deleted", r.Code, r.Body.String())
	}
	r = request(t, router, "PUT", path, body)
	if r.Code != 409 {
		t.Fatal("stale draft overwrote settings", r.Code)
	}
}
