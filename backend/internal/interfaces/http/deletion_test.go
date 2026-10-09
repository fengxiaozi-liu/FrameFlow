package http

import (
	"context"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"net/http/httptest"
	"testing"
)

type deletionVault struct{ keys map[string]string }

func (v *deletionVault) Put(_ context.Context, id, key string) error { v.keys[id] = key; return nil }
func (v *deletionVault) Has(_ context.Context, id string) (bool, error) {
	_, ok := v.keys[id]
	return ok, nil
}
func (v *deletionVault) Delete(_ context.Context, id string) error { delete(v.keys, id); return nil }
func TestDeletionEndpoints(t *testing.T) {
	s, done := testServer(t)
	defer done()
	ctx := context.Background()
	vault := &deletionVault{keys: map[string]string{"connection:test-connection": "secret"}}
	s.Providers.Catalog.Vault = vault
	router := testRouter(s)
	call := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("DELETE", path, nil))
		return w
	}
	if w := call("/api/projects/test-project/drafts/test-draft"); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("/api/projects/test-project/drafts/test-draft?expected_version=7"); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("/api/projects/test-project/drafts/missing?expected_version=0"); w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("/api/projects/test-project/drafts/test-draft?expected_version=0"); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	p, e := s.Projects.Repo.Get(ctx, "test-project")
	if e != nil || len(p.Drafts) != 0 {
		t.Fatal(p, e)
	}
	if _, e := s.Projects.Repo.(project.AtomicRepository).Update(ctx, "test-project", func(p *project.Project) error { return p.AddDraft(project.Draft{ID: "test-draft"}) }); e == nil {
		t.Fatal("deleted draft resurrected")
	}
	queued := task.Task{ID: "using-model", ModelID: "test-connection:story", Status: task.StatusQueued}
	if e = s.TaskService.Store.Save(ctx, queued); e != nil {
		t.Fatal(e)
	}
	if w := call("/api/connections/test-connection"); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	if ok, _ := vault.Has(ctx, "connection:test-connection"); !ok {
		t.Fatal("credential removed on conflict")
	}
	queued.Status = task.StatusFailed
	if e = s.TaskService.Store.Save(ctx, queued); e != nil {
		t.Fatal(e)
	}
	if w := call("/api/connections/test-connection"); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if ok, _ := vault.Has(ctx, "connection:test-connection"); ok {
		t.Fatal("credential retained")
	}
	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest("GET", "/api/models?connection_id=test-connection", nil))
	var result struct {
		Models []json.RawMessage `json:"models"`
	}
	if e = json.Unmarshal(list.Body.Bytes(), &result); e != nil || len(result.Models) != 0 {
		t.Fatal(e, list.Body.String())
	}
}
