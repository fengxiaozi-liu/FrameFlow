package http

import (
	"context"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"testing"
)

func TestEntityContextAtomicConflictAndIdentity(t *testing.T) {
	services, done := testServer(t)
	defer done()
	repo := services.Projects.Repo.(project.AtomicRepository)
	_, err := repo.Update(context.Background(), "test-project", func(p *project.Project) error {
		p.Drafts[0].Story.Scenes = []story.Scene{{ID: "s", Title: "shot", DurationSeconds: 4}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	router := testRouter(services)
	body := `{"expected_catalog_version":0,"entity":{"id":"c","kind":"character","name":"Ming","description":"blue jacket","revision":1,"references":[]}}`
	r := request(t, router, "POST", "/api/projects/test-project/entities", body)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	r = request(t, router, "POST", "/api/projects/test-project/entities", `{"expected_catalog_version":0,"entity":{"id":"d","kind":"character","name":"Yu","revision":1,"references":[]}}`)
	if r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	path := "/api/projects/test-project/drafts/test-draft/scenes/s/context"
	body = `{"expected_version":0,"entity_bindings":[{"id":"b","entity_id":"c","entity_revision":1,"role":"cast"}],"prompt_document":{"schema_version":1,"nodes":[{"type":"entity_ref","binding_id":"b"}]},"utterances":[]}`
	r = request(t, router, "PUT", path, body)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	var d project.Draft
	_ = json.Unmarshal(r.Body.Bytes(), &d)
	if d.Version != 1 || d.Story.Scenes[0].VisualPrompt != "Ming" {
		t.Fatal(d)
	}
	r = request(t, router, "PUT", path, body)
	if r.Code != 409 {
		t.Fatal(r.Code)
	}
	r = request(t, router, "PUT", path, `{"expected_version":1,"entity_bindings":[{"id":"b","entity_id":"outside","entity_revision":1,"role":"cast"}],"prompt_document":{"schema_version":1,"nodes":[]},"utterances":[]}`)
	if r.Code < 400 {
		t.Fatal("foreign entity accepted")
	}
	p, _ := repo.Get(context.Background(), "test-project")
	if p.Drafts[0].Version != 1 {
		t.Fatal("failed write changed revision")
	}
}
