package http

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

func TestCompositionSubmissionKeepsInputSnapshot(t *testing.T) {
	services, done := testServer(t)
	defer done()
	services.TaskService.Enqueuer = nil
	dir := t.TempDir()
	services.TaskService.UploadDir = dir
	services.TaskService.Probe = func(_ context.Context, path string) (task.MediaProbeResult, error) {
		if filepath.Base(path) == "clip.mp4" {
			return task.MediaProbeResult{Duration: 4.9, Width: 640, Height: 360, HasAudio: true}, nil
		}
		return task.MediaProbeResult{}, os.ErrNotExist
	}
	if err := os.WriteFile(filepath.Join(dir, "clip.mp4"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	repo := services.Projects.Repo.(project.AtomicRepository)
	var sceneID string
	_, err := repo.Update(context.Background(), "test-project", func(p *project.Project) error {
		d := &p.Drafts[0]
		if err := d.Story.AddScene(story.Scene{Title: "One", VisualPrompt: "Clip", DurationSeconds: 5}); err != nil {
			return err
		}
		sceneID = d.Story.Scenes[0].ID
		if err := d.AddVideoVersion(project.VideoVersion{ID: "version-one", SceneID: sceneID, TaskID: "video-task", LocalMediaPath: "/media/clip.mp4", DurationSeconds: 5, CreatedAt: time.Now()}); err != nil {
			return err
		}
		return d.SelectVideoVersion(sceneID, "version-one")
	})
	if err != nil {
		t.Fatal(err)
	}
	router := testRouter(services)
	body := `{"expected_version":0,"idempotency_key":"compose-1","aspect_ratio":"16:9","resolution":"720P","music_volume":0.2,"source_volume":1,"voice_volume":1}`
	submitted := request(t, router, http.MethodPost, "/api/projects/test-project/drafts/test-draft/compositions", body)
	if submitted.Code != http.StatusAccepted {
		t.Fatal(submitted.Code, submitted.Body.String())
	}
	var item task.Task
	if err := json.Unmarshal(submitted.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.Composition == nil || len(item.Composition.Clips) != 1 || item.Composition.Clips[0].VersionID != "version-one" || item.Composition.Clips[0].DurationSeconds != 4.9 {
		t.Fatal(item.Composition)
	}
	_, err = repo.Update(context.Background(), "test-project", func(p *project.Project) error {
		d := &p.Drafts[0]
		d.SelectedVersions[sceneID] = ""
		d.Story.Scenes[0].VisualPrompt = "changed"
		d.Version++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	item.ResultURL = "/media/composition-" + item.ID + ".mp4"
	if err := (application.TaskResultApplier{Projects: services.Projects.Repo}).Apply(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	p, err := services.Projects.Repo.Get(context.Background(), "test-project")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Drafts[0].Compositions) != 1 || p.Drafts[0].Compositions[0].ClipVersionIDs[0] != "version-one" || p.Drafts[0].SelectedVersions[sceneID] != "" {
		t.Fatal(p.Drafts[0])
	}
}
