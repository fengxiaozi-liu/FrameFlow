package http

import (
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"testing"
)

func TestCharacterVoiceMaterialValidationAndHistory(t *testing.T) {
	s, done := testServer(t)
	defer done()
	router := testRouter(s)
	for _, body := range []string{
		`{"id":"voice-a","name":"voice","kind":"voice","url":"/media/a.wav"}`,
		`{"id":"music-a","name":"music","kind":"music","url":"/media/b.wav"}`,
	} {
		if w := request(t, router, "POST", "/api/materials", body); w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	base := "/api/projects/test-project/entities"
	for _, pair := range [][2]string{{"character", "missing"}, {"character", "music-a"}, {"location", "voice-a"}} {
		body := map[string]any{"expected_catalog_version": 0, "entity": map[string]any{"id": "actor", "name": "actor", "kind": pair[0], "revision": 1, "references": []any{}, "voice_material_id": pair[1]}}
		raw, _ := json.Marshal(body)
		if w := request(t, router, "POST", base, string(raw)); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	first := request(t, router, "POST", base, `{"expected_catalog_version":0,"entity":{"id":"actor","name":"actor","kind":"character","revision":1,"references":[],"voice_material_id":"voice-a"}}`)
	if first.Code != 200 {
		t.Fatal(first.Code, first.Body.String())
	}
	var catalog project.EntityCatalog
	if err := json.Unmarshal(first.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Entities[0].VoiceMaterialID != "voice-a" || len(catalog.Voices) != 0 {
		t.Fatal(catalog)
	}
	cleared := request(t, router, "PATCH", base+"/actor", `{"expected_catalog_version":1,"entity":{"id":"actor","name":"actor","kind":"character","revision":2,"references":[]}}`)
	if cleared.Code != 200 {
		t.Fatal(cleared.Code, cleared.Body.String())
	}
	if w := request(t, router, "DELETE", "/api/materials/voice-a", ""); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
}
