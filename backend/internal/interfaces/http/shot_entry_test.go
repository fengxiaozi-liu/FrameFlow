package http

import (
	"context"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"testing"
)

func TestSceneCreateRecoveryAndNameValidation(t *testing.T) {
	s, done := testServer(t)
	defer done()
	router := testRouter(s)
	base := "/api/projects/test-project/drafts/test-draft"
	body := `{"expected_version":0,"action":"create","title":"未命名分镜","duration_seconds":4,"creation_id":"scene-created-01234567-0123-0123-0123-012345678901"}`
	first := request(t, router, "POST", base+"/scenes", body)
	if first.Code != 201 {
		t.Fatal(first.Code, first.Body.String())
	}
	var d project.Draft
	if err := json.Unmarshal(first.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	scene := d.Story.Scenes[len(d.Story.Scenes)-1]
	renamed := request(t, router, "PUT", base+"/scenes/"+scene.ID+"/context", `{"expected_version":1,"title":"雨夜咖啡馆","entity_bindings":[],"utterances":[],"prompt_document":{"schema_version":1,"nodes":[]}}`)
	if renamed.Code != 200 {
		t.Fatal(renamed.Code, renamed.Body.String())
	}
	replay := request(t, router, "POST", base+"/scenes", body)
	if replay.Code != 200 {
		t.Fatal(replay.Code, replay.Body.String())
	}
	var recovered project.Draft
	if err := json.Unmarshal(replay.Body.Bytes(), &recovered); err != nil {
		t.Fatal(err)
	}
	if len(recovered.Story.Scenes) != len(d.Story.Scenes) || recovered.Version != 2 || recovered.Story.Scenes[len(d.Story.Scenes)-1].Title != "雨夜咖啡馆" {
		t.Fatal(recovered)
	}
	invalid := request(t, router, "PUT", base+"/scenes/"+scene.ID+"/context", `{"expected_version":2,"title":"  ","entity_bindings":[],"utterances":[],"prompt_document":{"schema_version":1,"nodes":[]}}`)
	if invalid.Code != 400 {
		t.Fatal(invalid.Code, invalid.Body.String())
	}
	_, err := s.Projects.Repo.(project.AtomicRepository).Update(context.Background(), "test-project", func(p *project.Project) error {
		p.EntityCatalog.Entities = []project.EntityVersion{{ID: "actor", Kind: "character", Name: "林夏", Revision: 1}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	legacyBody := `{"expected_version":2,"title":"雨夜咖啡馆","entity_bindings":[{"id":"b","entity_id":"actor","entity_revision":1,"role":"cast","description_supplement":"湿发"}],"utterances":[],"prompt_document":{"schema_version":1,"nodes":[{"type":"text","text":"雨夜"}]}}`
	migrated := request(t, router, "PUT", base+"/scenes/"+scene.ID+"/context", legacyBody)
	if migrated.Code != 200 {
		t.Fatal(migrated.Code, migrated.Body.String())
	}
	var saved project.Draft
	if err := json.Unmarshal(migrated.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	current := saved.Story.Scenes[len(saved.Story.Scenes)-1]
	if current.VisualPrompt != "雨夜\n林夏：湿发" || current.EntityBindings[0].DescriptionSupplement != "" {
		t.Fatal(current)
	}
}
