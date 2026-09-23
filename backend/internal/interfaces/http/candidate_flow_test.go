package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
)

func TestCandidateApplyConflictAndStoryboardRestore(t *testing.T) {
	services, done := testServer(t)
	defer done()
	repo := services.Projects.Repo.(project.AtomicRepository)
	_, err := repo.Update(context.Background(), "test-project", func(p *project.Project) error {
		d := &p.Drafts[0]
		d.Story.Body = "original"
		_ = d.Story.AddScene(story.Scene{Title: "old", VisualPrompt: "old frame", DurationSeconds: 5})
		oldSceneID := d.Story.Scenes[0].ID
		d.Bindings = []project.SceneBinding{{SceneID: oldSceneID, MaterialID: "old-material", Usage: "first_frame"}}
		d.VideoVersions = []project.VideoVersion{{ID: "old-version", SceneID: oldSceneID, TaskID: "old-task", LocalMediaPath: "/media/old.mp4"}}
		d.SelectedVersions = map[string]string{oldSceneID: "old-version"}
		d.Candidates = []story.Candidate{
			{ID: "body-candidate", Target: story.CandidateStory, SourceVersion: 0, Body: "generated", Status: "preview", CreatedAt: time.Now()},
			{ID: "board-candidate", Target: story.CandidateStoryboard, SourceVersion: 0, SourceBody: "original", Scenes: []story.Scene{{Title: "new", VisualPrompt: "new frame", DurationSeconds: 5}}, Status: "preview", CreatedAt: time.Now()},
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	router := testRouter(services)
	path := "/api/projects/test-project/drafts/test-draft"
	applied := request(t, router, http.MethodPost, path+"/candidates/body-candidate/apply", `{"expected_version":0,"action":"append"}`)
	if applied.Code != http.StatusOK {
		t.Fatal(applied.Code, applied.Body.String())
	}
	var draft project.Draft
	if err := json.Unmarshal(applied.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	if draft.Story.Body != "original\n\ngenerated" || len(draft.BodySnapshots) != 1 {
		t.Fatalf("body apply: %+v", draft)
	}
	stale := request(t, router, http.MethodPost, path+"/candidates/board-candidate/apply", `{"expected_version":0,"action":"replace"}`)
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale apply: %d %s", stale.Code, stale.Body.String())
	}
	review := request(t, router, http.MethodPost, path+"/candidates/board-candidate/apply", `{"expected_version":1,"action":"replace"}`)
	if review.Code != http.StatusConflict {
		t.Fatalf("review required: %d %s", review.Code, review.Body.String())
	}
	confirmed := request(t, router, http.MethodPost, path+"/candidates/board-candidate/apply", `{"expected_version":1,"action":"replace","reconfirm":true}`)
	if confirmed.Code != http.StatusOK {
		t.Fatal(confirmed.Code, confirmed.Body.String())
	}
	if err := json.Unmarshal(confirmed.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	if len(draft.Story.Scenes) != 1 || draft.Story.Scenes[0].Title != "new" || len(draft.StoryboardSnapshots) != 1 {
		t.Fatalf("board apply: %+v", draft)
	}
	restored := request(t, router, http.MethodPost, path+"/restore", `{"expected_version":2,"snapshot_id":"board-board-candidate"}`)
	if restored.Code != http.StatusOK {
		t.Fatal(restored.Code, restored.Body.String())
	}
	if err := json.Unmarshal(restored.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	if len(draft.Story.Scenes) != 1 || draft.Story.Scenes[0].Title != "old" {
		t.Fatalf("restore: %+v", draft.Story.Scenes)
	}
	if len(draft.Bindings) != 1 || draft.Bindings[0].SceneID != draft.Story.Scenes[0].ID || len(draft.VideoVersions) != 1 || draft.SelectedVersions[draft.Story.Scenes[0].ID] != "old-version" {
		t.Fatalf("restore must recover bindings and selected video: %+v", draft)
	}
}

func TestSceneBindingRequiresConfirmedMaterialAndVersion(t *testing.T) {
	services, done := testServer(t)
	defer done()
	ctx := context.Background()
	if err := services.Materials.Repo.Save(ctx, material.Asset{ID: "scene-image", Name: "Scene", Kind: material.Scene, URL: "https://example.com/a.png", Media: material.MediaInfo{Format: "image/png", SizeBytes: 1024}}); err != nil {
		t.Fatal(err)
	}
	repo := services.Projects.Repo.(project.AtomicRepository)
	_, err := repo.Update(ctx, "test-project", func(p *project.Project) error {
		return p.Drafts[0].Story.AddScene(story.Scene{Title: "one", VisualPrompt: "frame", DurationSeconds: 5})
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := services.Projects.Repo.Get(ctx, "test-project")
	if err != nil {
		t.Fatal(err)
	}
	sceneID := p.Drafts[0].Story.Scenes[0].ID
	router := testRouter(services)
	path := "/api/projects/test-project/drafts/test-draft/scenes/" + sceneID + "/bindings"
	invalid := request(t, router, http.MethodPut, path, `{"expected_version":0,"bindings":[{"material_id":"missing","usage":"first_frame"}]}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatal(invalid.Code, invalid.Body.String())
	}
	confirmed := request(t, router, http.MethodPut, path, `{"expected_version":0,"bindings":[{"material_id":"scene-image","usage":"first_frame"}]}`)
	if confirmed.Code != http.StatusOK {
		t.Fatal(confirmed.Code, confirmed.Body.String())
	}
	stale := request(t, router, http.MethodPut, path, `{"expected_version":0,"bindings":[]}`)
	if stale.Code != http.StatusConflict {
		t.Fatal(stale.Code, stale.Body.String())
	}
	p, err = services.Projects.Repo.Get(ctx, "test-project")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Drafts[0].Bindings) != 1 || p.Drafts[0].Bindings[0].SceneID != sceneID {
		t.Fatal(p.Drafts[0].Bindings)
	}
	blocked := request(t, router, http.MethodDelete, "/api/materials/scene-image", "")
	if blocked.Code != http.StatusConflict || !strings.Contains(blocked.Body.String(), "material_in_use") || !strings.Contains(blocked.Body.String(), sceneID) {
		t.Fatalf("bound material should be protected with usage: %d %s", blocked.Code, blocked.Body.String())
	}
}

func TestConcurrentCandidateApplicationKeepsSingleRevision(t *testing.T) {
	services, done := testServer(t)
	defer done()
	repo := services.Projects.Repo.(project.AtomicRepository)
	_, err := repo.Update(context.Background(), "test-project", func(p *project.Project) error {
		p.Drafts[0].Candidates = []story.Candidate{{ID: "candidate", Target: story.CandidateStory, SourceVersion: 0, Body: "generated", Status: "preview"}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	router := testRouter(services)
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := request(t, router, http.MethodPost, "/api/projects/test-project/drafts/test-draft/candidates/candidate/apply", `{"expected_version":0,"action":"append"}`)
			results <- response.Code
		}()
	}
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for code := range results {
		counts[code]++
	}
	if counts[http.StatusOK] != 1 || counts[http.StatusConflict] != 1 {
		t.Fatalf("concurrent apply results: %+v", counts)
	}
	p, err := services.Projects.Repo.Get(context.Background(), "test-project")
	if err != nil {
		t.Fatal(err)
	}
	if p.Drafts[0].Version != 1 || p.Drafts[0].Story.Body != "generated" || len(p.Drafts[0].BodySnapshots) != 1 {
		t.Fatalf("concurrent apply changed draft twice: %+v", p.Drafts[0])
	}
}
