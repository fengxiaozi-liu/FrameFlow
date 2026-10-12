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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestReferenceVideoRunSingleStepAndFrozenInputs(t *testing.T) {
	s, done := testServer(t)
	defer done()
	ctx := context.Background()
	s.TaskService.UploadDir = t.TempDir()
	s.TaskService.Enqueuer = nil
	if err := s.Providers.Catalog.Models.SaveModel(ctx, provider.Model{ID: "r2v", ConnectionID: "test-connection", RemoteID: "wan2.7-r2v", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Video}}); err != nil {
		t.Fatal(err)
	}
	refs := []project.ReferenceImage{}
	for i := 0; i < 6; i++ {
		id := fmt.Sprintf("ref%d", i)
		if err := os.WriteFile(filepath.Join(s.TaskService.UploadDir, id+".png"), []byte(id), 0600); err != nil {
			t.Fatal(err)
		}
		if err := s.TaskService.Materials.Save(ctx, material.Asset{ID: id, Kind: material.Scene, URL: "/media/" + id + ".png"}); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, project.ReferenceImage{MaterialID: id, Order: i})
	}
	setReferences := func(n int) {
		t.Helper()
		_, err := s.Projects.Repo.(project.AtomicRepository).Update(ctx, "test-project", func(p *project.Project) error {
			p.EntityCatalog.Entities = []project.EntityVersion{{ID: "cafe", Kind: "location", Name: "咖啡馆", Description: "warm light", Revision: 1, References: refs[:n]}}
			p.Drafts[0].Story.Scenes = []story.Scene{{ID: "s", VisualPrompt: "original scene", Narration: "rain", DurationSeconds: 6, EntityBindings: []story.EntityBinding{{ID: "b", EntityID: "cafe", EntityRevision: 1, Role: "primary"}}}}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	router := testRouter(s)
	path := "/api/projects/test-project/drafts/test-draft/scenes/s/generation-runs"
	body := `{"expected_version":0,"idempotency_key":"direct","model_id":"r2v","video_model_id":"r2v","path":"reference_video","aspect_ratio":"9:16","duration_seconds":6,"resolution":"720P"}`
	for _, n := range []int{0, 6} {
		setReferences(n)
		r := request(t, router, "POST", path, body)
		if r.Code != 400 {
			t.Fatal(n, r.Code, r.Body.String())
		}
	}
	setReferences(2)
	var wg sync.WaitGroup
	codes := make(chan int, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- request(t, router, "POST", path, body).Code }()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 202 {
			t.Fatal(code)
		}
	}
	response := request(t, router, "POST", path, body)
	var run task.GenerationRun
	if err := json.Unmarshal(response.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	steps, err := s.TaskService.Store.List(ctx)
	if err != nil || len(steps) != 1 {
		t.Fatal(len(steps), err)
	}
	step := steps[0]
	if step.Kind != task.KindVideo || len(step.DependsOn) != 0 || step.Snapshot == nil || len(step.Snapshot.Entities[0].References) != 2 || len(run.Plan) != 1 || !strings.Contains(step.Snapshot.Prompt, "旁白：rain") {
		t.Fatalf("wrong direct task: %+v", step)
	}
	originalHash := step.InputHash
	_, err = s.Projects.Repo.(project.AtomicRepository).Update(ctx, "test-project", func(p *project.Project) error {
		p.Drafts[0].Story.Scenes[0].VisualPrompt = "changed scene"
		p.EntityCatalog.Entities[0].Description = "changed"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		r := request(t, router, "GET", path+"/"+run.ID, "")
		if r.Code != 200 {
			t.Fatal(r.Body.String())
		}
		_ = json.Unmarshal(r.Body.Bytes(), &run)
	}
	if run.State != "generating_video" || len(run.Plan) != 1 || run.Plan[0].InputHash != originalHash || strings.Contains(run.Plan[0].Prompt, "changed") {
		t.Fatal(run)
	}
	step.Status = task.StatusSucceeded
	if err := s.TaskService.Store.Save(ctx, step); err != nil {
		t.Fatal(err)
	}
	_, err = s.Projects.Repo.(project.AtomicRepository).Update(ctx, "test-project", func(p *project.Project) error {
		p.Drafts[0].VideoVersions = []project.VideoVersion{{ID: "version-" + step.ID, TaskID: step.ID, SceneID: "s", LocalMediaPath: "/media/result.mp4"}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	response = request(t, router, "PUT", path+"/"+run.ID+"/selected-media", fmt.Sprintf(`{"expected_version":0,"video_version_id":%q}`, "version-"+step.ID))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	_ = json.Unmarshal(response.Body.Bytes(), &run)
	if run.State != "awaiting_composition" {
		t.Fatal(run.State)
	}
	all, _ := s.TaskService.Store.List(ctx)
	if len(all) != 1 {
		t.Fatal("created unwanted image task", len(all))
	}
	// New video audio must not be blocked or replaced by the legacy TTS flow.
	_, err = s.Projects.Repo.(project.AtomicRepository).Update(ctx, "test-project", func(p *project.Project) error {
		p.Drafts[0].Story.Scenes[0].Utterances = []story.Utterance{{ID: "line", Text: "hello", SpeakerID: "actor", SpeakerRevision: 1}}
		p.Drafts[0].Bindings = append(p.Drafts[0].Bindings, project.SceneBinding{SceneID: "s", Usage: "voiceover", MaterialID: "old-voice"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	s.TaskService.Probe = func(context.Context, string) (task.MediaProbeResult, error) {
		return task.MediaProbeResult{Duration: 6, Width: 1280, Height: 720, HasAudio: true}, nil
	}
	if err := os.WriteFile(filepath.Join(s.TaskService.UploadDir, "result.mp4"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	router = testRouter(s)
	composeBody := `{"expected_version":1,"idempotency_key":"compose-direct","aspect_ratio":"16:9","resolution":"720P","source_volume":1}`
	response = request(t, router, "POST", "/api/projects/test-project/drafts/test-draft/compositions", composeBody)
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	var composition task.Task
	if err := json.Unmarshal(response.Body.Bytes(), &composition); err != nil {
		t.Fatal(err)
	}
	if composition.Composition == nil || len(composition.Composition.Clips) != 1 {
		t.Fatal(composition)
	}
	clip := composition.Composition.Clips[0]
	if clip.MuteSource || len(clip.VoiceSegments) != 0 || clip.VoicePath != "" {
		t.Fatal("reference video audio overwritten", clip)
	}
	// The same draft with a historical video continues to require its TTS takes.
	legacy := step
	legacy.ID, legacy.RunID, legacy.Snapshot = "legacy-video", "", nil
	if err := s.TaskService.Store.Save(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	_, err = s.Projects.Repo.(project.AtomicRepository).Update(ctx, "test-project", func(p *project.Project) error {
		p.Drafts[0].VideoVersions[0].TaskID = legacy.ID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	response = request(t, router, "POST", "/api/projects/test-project/drafts/test-draft/compositions", strings.ReplaceAll(composeBody, "compose-direct", "compose-legacy"))
	if response.Code != 422 || !strings.Contains(response.Body.String(), "utterance:line") {
		t.Fatal(response.Code, response.Body.String())
	}
}
