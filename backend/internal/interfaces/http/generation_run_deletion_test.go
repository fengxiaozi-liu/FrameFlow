package http

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeleteFailedRunAtomicAndCannotResurrect(t *testing.T) {
	s, done := testServer(t)
	defer done()
	ctx := context.Background()
	p, err := s.Projects.Repo.Get(ctx, "test-project")
	if err != nil {
		t.Fatal(err)
	}
	d := &p.Drafts[0]
	d.Version = 3
	d.Story.Scenes = []story.Scene{{ID: "s"}}
	d.GenerationRuns = []task.GenerationRun{{ID: "failed", SceneID: "s", State: "failed", TaskIDs: []string{"failed-0", "failed-1"}, Plan: []task.GenerationSnapshot{{}, {}, {}}}, {ID: "other", SceneID: "s", State: "generating"}}
	d.VideoVersions = []project.VideoVersion{{ID: "v", TaskID: "failed-0", SceneID: "s", LocalMediaPath: "/media/keep.mp4"}}
	d.SelectedVersions = map[string]string{"s": "v"}
	if err = s.Projects.Repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}
	children := []task.Task{{ID: "failed-0", Status: task.StatusSucceeded}, {ID: "failed-1", Status: task.StatusFailed}, {ID: "extra-child", Status: task.StatusCancelled}}
	children[0].Snapshot = &task.GenerationSnapshot{ID: "snap-video", Path: "reference_video"}
	for i := range children {
		child := &children[i]
		child.ProjectID = p.ID
		child.DraftID = d.ID
		child.SceneID = "s"
		child.RunID = "failed"
		child.Kind = task.KindVideo
		if err = s.TaskService.Store.Save(ctx, *child); err != nil {
			t.Fatal(err)
		}
	}
	router := testRouter(s)
	path := "/api/projects/test-project/drafts/test-draft/scenes/s/generation-runs/failed"
	if r := request(t, router, "DELETE", "/api/projects/test-project/drafts/test-draft/scenes/other/generation-runs/failed", ""); r.Code != 404 {
		t.Fatal(r.Code, r.Body.String())
	}
	// The task-center endpoint must remove the whole run, never orphan one step.
	if r := request(t, router, "DELETE", "/api/tasks/failed-1", ""); r.Code != 204 {
		t.Fatal(r.Code, r.Body.String())
	}
	fresh, _ := s.Projects.Repo.Get(ctx, p.ID)
	updated := fresh.Drafts[0]
	if len(updated.GenerationRuns) != 1 || updated.GenerationRuns[0].ID != "other" || updated.Version != 4 || len(updated.VideoVersions) != 1 || updated.SelectedVersions["s"] != "v" || updated.VideoVersions[0].GenerationPath != "reference_video" {
		t.Fatalf("history/media changed: %+v", updated)
	}
	for _, child := range children {
		if _, err = s.TaskService.Store.Get(ctx, child.ID); !errors.Is(err, fault.ErrNotFound) {
			t.Fatal("child retained", child.ID, err)
		}
		if err = s.TaskService.Store.Save(ctx, child); err == nil {
			t.Fatal("late save resurrected task")
		}
		if _, _, err = s.TaskService.Store.(interface {
			CreateIfAbsent(context.Context, task.Task) (task.Task, bool, error)
		}).CreateIfAbsent(ctx, child); err == nil {
			t.Fatal("duplicate submission resurrected task")
		}
	}
	missing := children[1]
	missing.ID = "failed-2"
	if err = s.TaskService.Store.Save(ctx, missing); err == nil {
		t.Fatal("uncreated planned step resurrected")
	}
	if err = s.Projects.Repo.Save(ctx, p); err == nil {
		t.Fatal("stale project resurrected run")
	}
	applier := application.TaskResultApplier{Projects: s.Projects.Repo}
	late := children[0]
	late.ResultURL = "/media/late.mp4"
	late.Input.Duration = 5
	if err = applier.Apply(ctx, late); !errors.Is(err, fault.ErrNotFound) {
		t.Fatal("late result applied", err)
	}
	s.TaskService.AdvanceGenerationRuns(ctx)
	if r := request(t, router, "DELETE", path, ""); r.Code != 404 {
		t.Fatal(r.Code, r.Body.String())
	}
	fresh, _ = s.Projects.Repo.Get(ctx, p.ID)
	if len(fresh.Drafts[0].VideoVersions) != 1 {
		t.Fatal("late result changed media")
	}
	// Dialogue was embedded in this source video; removing history must not
	// suddenly require separate TTS takes to compose the retained version.
	s.TaskService.UploadDir = t.TempDir()
	if err = os.WriteFile(filepath.Join(s.TaskService.UploadDir, "keep.mp4"), []byte("local test clip"), 0600); err != nil {
		t.Fatal(err)
	}
	s.TaskService.Probe = func(context.Context, string) (task.MediaProbeResult, error) {
		return task.MediaProbeResult{Duration: 5, Width: 720, Height: 1280, HasAudio: true}, nil
	}
	s.TaskService.Enqueuer = nil
	_, err = s.Projects.Repo.(project.AtomicRepository).Update(ctx, p.ID, func(p *project.Project) error {
		p.Drafts[0].Story.Scenes[0].Utterances = []story.Utterance{{ID: "u", SpeakerType: "narrator", Text: "hello"}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if response := request(t, testRouter(s), "POST", "/api/projects/test-project/drafts/test-draft/compositions", `{"expected_version":4,"idempotency_key":"retained-video","aspect_ratio":"9:16","resolution":"720P","source_volume":1,"music_volume":0,"voice_volume":0}`); response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
}

func TestDeleteRunRejectsStateChangesWithoutPartialRemoval(t *testing.T) {
	s, done := testServer(t)
	defer done()
	ctx := context.Background()
	p, _ := s.Projects.Repo.Get(ctx, "test-project")
	p.Drafts[0].Story.Scenes = []story.Scene{{ID: "s"}}
	p.Drafts[0].GenerationRuns = []task.GenerationRun{{ID: "r", SceneID: "s", State: "failed", TaskIDs: []string{"r-0"}}}
	if err := s.Projects.Repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}
	child := task.Task{ID: "r-0", ProjectID: p.ID, DraftID: p.Drafts[0].ID, SceneID: "s", RunID: "r", Status: task.StatusRunning, Kind: task.KindVideo}
	if err := s.TaskService.Store.Save(ctx, child); err != nil {
		t.Fatal(err)
	}
	router := testRouter(s)
	path := "/api/projects/test-project/drafts/test-draft/scenes/s/generation-runs/r"
	if r := request(t, router, "DELETE", path, ""); r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	fresh, _ := s.Projects.Repo.Get(ctx, p.ID)
	if len(fresh.Drafts[0].GenerationRuns) != 1 || fresh.Drafts[0].Version != p.Drafts[0].Version {
		t.Fatal("partial removal")
	}
	child.Status = task.StatusFailed
	if err := s.TaskService.Store.Save(ctx, child); err != nil {
		t.Fatal(err)
	}
	if r := request(t, router, "DELETE", path, ""); r.Code != 204 {
		t.Fatal(r.Code, r.Body.String())
	}
	// Standalone deletion also rejects active jobs and terminal late writes.
	single := task.Task{ID: "single", ProjectID: p.ID, DraftID: p.Drafts[0].ID, Kind: task.KindVideo, Status: task.StatusRunning, CreatedAt: time.Now()}
	if err := s.TaskService.Store.Save(ctx, single); err != nil {
		t.Fatal(err)
	}
	if r := request(t, router, "DELETE", "/api/tasks/single", ""); r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	single.Status = task.StatusFailed
	if err := s.TaskService.Store.Save(ctx, single); err != nil {
		t.Fatal(err)
	}
	if r := request(t, router, "DELETE", "/api/tasks/single", ""); r.Code != 204 {
		t.Fatal(r.Code, r.Body.String())
	}
	if err := s.TaskService.Store.Save(ctx, single); err == nil {
		t.Fatal("standalone task resurrected")
	}
}
