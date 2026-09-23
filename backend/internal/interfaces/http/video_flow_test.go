package http

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

func TestSceneVideoIdempotencyAndVersionSelection(t *testing.T) {
	services, done := testServer(t)
	defer done()
	services.TaskService.Enqueuer = nil
	dir := t.TempDir()
	services.TaskService.UploadDir = dir
	services.UploadDir = dir
	if err := os.WriteFile(filepath.Join(dir, "first.png"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	model := provider.Model{ID: "wan-test", ConnectionID: "test-connection", RemoteID: "wan2.7-i2v", Name: "Wan", Capabilities: []provider.Capability{provider.Video}, Supported: true, Enabled: true}
	if err := services.Providers.Catalog.Models.SaveModel(ctx, model); err != nil {
		t.Fatal(err)
	}
	if err := services.Materials.Repo.Save(ctx, material.Asset{ID: "first", Name: "First", Kind: material.Scene, URL: "/media/first.png", Media: material.MediaInfo{Format: "image/png", SizeBytes: 100, Width: 640, Height: 480}}); err != nil {
		t.Fatal(err)
	}
	repo := services.Projects.Repo.(project.AtomicRepository)
	var sceneID string
	_, err := repo.Update(ctx, "test-project", func(p *project.Project) error {
		d := &p.Drafts[0]
		if err := d.Story.AddScene(story.Scene{Title: "One", VisualPrompt: "A moving scene", DurationSeconds: 5}); err != nil {
			return err
		}
		sceneID = d.Story.Scenes[0].ID
		return d.AddBinding(project.SceneBinding{SceneID: sceneID, MaterialID: "first", Usage: "first_frame"})
	})
	if err != nil {
		t.Fatal(err)
	}
	router := testRouter(services)
	path := "/api/projects/test-project/drafts/test-draft/scenes/" + sceneID
	unknown := request(t, router, http.MethodPost, path+"/video-validation", `{"model_id":"missing-model","resolution":"720P"}`)
	if unknown.Code != http.StatusOK || !strings.Contains(unknown.Body.String(), `"field":"model_id"`) || !strings.Contains(unknown.Body.String(), `"scene_id":"`+sceneID+`"`) {
		t.Fatalf("unknown model must identify scene and field: %d %s", unknown.Code, unknown.Body.String())
	}
	if err := os.WriteFile(filepath.Join(dir, "voice.mp3"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := services.Materials.Repo.Save(ctx, material.Asset{ID: "audio", Name: "Voice", Kind: material.Voice, URL: "/media/voice.mp3", Media: material.MediaInfo{Format: "audio/mpeg", SizeBytes: 100, DurationSeconds: 5}}); err != nil {
		t.Fatal(err)
	}
	_, err = repo.Update(ctx, "test-project", func(p *project.Project) error {
		return p.Drafts[0].AddBinding(project.SceneBinding{SceneID: sceneID, MaterialID: "audio", Usage: "driving_audio"})
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("FRAMEFLOW_AUDIO_OBJECT_BASE_URL", "")
	withAudio := request(t, router, http.MethodPost, path+"/video-validation", `{"model_id":"wan-test","resolution":"720P"}`)
	if withAudio.Code != http.StatusOK || !strings.Contains(withAudio.Body.String(), `"field":"driving_audio"`) || !strings.Contains(withAudio.Body.String(), "未配置音频对象存储") {
		t.Fatalf("local audio without OSS must degrade with clear error: %d %s", withAudio.Code, withAudio.Body.String())
	}
	_, err = repo.Update(ctx, "test-project", func(p *project.Project) error { p.Drafts[0].Bindings = p.Drafts[0].Bindings[:1]; return nil })
	if err != nil {
		t.Fatal(err)
	}
	body := `{"expected_version":0,"idempotency_key":"submit-1","model_id":"wan-test","resolution":"720P"}`
	first := request(t, router, http.MethodPost, path+"/video-tasks", body)
	if first.Code != http.StatusAccepted {
		t.Fatal(first.Code, first.Body.String())
	}
	var created task.Task
	if err := json.Unmarshal(first.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	repeated := request(t, router, http.MethodPost, path+"/video-tasks", body)
	var duplicate task.Task
	if err := json.Unmarshal(repeated.Body.Bytes(), &duplicate); err != nil {
		t.Fatal(err)
	}
	if repeated.Code != http.StatusAccepted || duplicate.ID != created.ID || created.Input.SourceImageURL != "/media/first.png" {
		t.Fatalf("idempotency: %d %+v %+v", repeated.Code, created, duplicate)
	}
	created.ResultURL = "/media/generated-" + created.ID + ".mp4"
	if err := (application.TaskResultApplier{Projects: services.Projects.Repo}).Apply(ctx, created); err != nil {
		t.Fatal(err)
	}
	if err := (application.TaskResultApplier{Projects: services.Projects.Repo}).Apply(ctx, created); err != nil {
		t.Fatal(err)
	}
	p, err := services.Projects.Repo.Get(ctx, "test-project")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Drafts[0].VideoVersions) != 1 || p.Drafts[0].SelectedVersions[sceneID] != "" {
		t.Fatal(p.Drafts[0].VideoVersions, p.Drafts[0].SelectedVersions)
	}
	version := p.Drafts[0].VideoVersions[0]
	selection := request(t, router, http.MethodPut, path+"/selected-version", `{"expected_version":0,"version_id":"`+version.ID+`"}`)
	if selection.Code != http.StatusOK {
		t.Fatal(selection.Code, selection.Body.String())
	}
	stale := request(t, router, http.MethodPut, path+"/selected-version", `{"expected_version":0,"version_id":""}`)
	if stale.Code != http.StatusConflict {
		t.Fatal(stale.Code, stale.Body.String())
	}
	p, err = services.Projects.Repo.Get(ctx, "test-project")
	if err != nil {
		t.Fatal(err)
	}
	if p.Drafts[0].SelectedVersions[sceneID] != version.ID {
		t.Fatal(p.Drafts[0].SelectedVersions)
	}
	changed := request(t, router, http.MethodPatch, path, `{"expected_version":1,"action":"update","title":"One","visual_prompt":"Changed motion","duration_seconds":5}`)
	if changed.Code != http.StatusOK {
		t.Fatal(changed.Code, changed.Body.String())
	}
	listed := request(t, router, http.MethodGet, path+"/versions", "")
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"based_on_old_settings":true`) {
		t.Fatal(listed.Code, listed.Body.String())
	}
}

func TestLateVideoResultCannotCrossDraftOrRecreateDeletedScene(t *testing.T) {
	services, done := testServer(t)
	defer done()
	ctx := context.Background()
	repo := services.Projects.Repo.(project.AtomicRepository)
	_, err := repo.Update(ctx, "test-project", func(p *project.Project) error {
		if err := p.AddDraft(project.Draft{ID: "other-draft", Name: "Other"}); err != nil {
			return err
		}
		if err := p.Drafts[0].Story.AddScene(story.Scene{ID: "deleted-scene", Title: "Deleted", DurationSeconds: 5}); err != nil {
			return err
		}
		if err := p.Drafts[1].Story.AddScene(story.Scene{ID: "other-scene", Title: "Other", DurationSeconds: 5}); err != nil {
			return err
		}
		return p.Drafts[1].AddVideoVersion(project.VideoVersion{ID: "selected", SceneID: "other-scene", TaskID: "previous", LocalMediaPath: "/media/previous.mp4"})
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Update(ctx, "test-project", func(p *project.Project) error {
		if err := p.Drafts[1].SelectVideoVersion("other-scene", "selected"); err != nil {
			return err
		}
		p.Drafts[0].Story.Scenes = nil
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	applier := application.TaskResultApplier{Projects: services.Projects.Repo}
	late := task.Task{ID: "late", ProjectID: "test-project", DraftID: "test-draft", SceneID: "deleted-scene", Kind: task.KindVideo, ResultURL: "/media/late.mp4"}
	if err := applier.Apply(ctx, late); err != nil {
		t.Fatal(err)
	}
	other := task.Task{ID: "new", ProjectID: "test-project", DraftID: "other-draft", SceneID: "other-scene", Kind: task.KindVideo, ResultURL: "/media/new.mp4"}
	if err := applier.Apply(ctx, other); err != nil {
		t.Fatal(err)
	}
	if err := applier.Apply(ctx, other); err != nil {
		t.Fatal(err)
	}
	p, err := services.Projects.Repo.Get(ctx, "test-project")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Drafts[0].VideoVersions) != 0 || len(p.Drafts[1].VideoVersions) != 2 || p.Drafts[1].SelectedVersions["other-scene"] != "selected" {
		t.Fatalf("late or repeated callback changed another draft or selection: %+v", p.Drafts)
	}
}
