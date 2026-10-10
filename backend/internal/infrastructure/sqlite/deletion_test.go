package sqlite

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"path/filepath"
	"testing"
)

func deletionStore(t *testing.T) *TaskRepository {
	t.Helper()
	r, e := Open(context.Background(), filepath.Join(t.TempDir(), "delete.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close() })
	return r
}
func requireDeleteErr(t *testing.T, e error, code string) {
	t.Helper()
	var f *fault.Error
	if !errors.As(e, &f) || f.Code != code {
		t.Fatalf("expected %s, got %v", code, e)
	}
}
func TestDraftDeletionIsolationVersionAndLateWrites(t *testing.T) {
	ctx := context.Background()
	r := deletionStore(t)
	repo := NewProjectRepository(r)
	p := project.Project{ID: "p", Name: "project", Drafts: []project.Draft{{ID: "a", Name: "a", Version: 2}, {ID: "b", Name: "b", Version: 7}}}
	if e := r.SaveMaterial(ctx, material.Asset{ID: "shared", Kind: material.Kind("scene")}); e != nil {
		t.Fatal(e)
	}
	p.Drafts[0].Bindings = []project.SceneBinding{{SceneID: "s-a", MaterialID: "shared", Usage: "scene"}}
	p.Drafts[1].Bindings = []project.SceneBinding{{SceneID: "s-b", MaterialID: "shared", Usage: "scene"}}
	if e := repo.Save(ctx, p); e != nil {
		t.Fatal(e)
	}
	if e := repo.DeleteDraft(ctx, "other", "a", 2); !errors.Is(e, fault.ErrNotFound) {
		t.Fatal(e)
	}
	requireDeleteErr(t, repo.DeleteDraft(ctx, "p", "a", 1), "draft_version_conflict")
	taskA := task.Task{ID: "task", ProjectID: "p", DraftID: "a", Status: task.StatusRunning}
	if e := r.Save(ctx, taskA); e != nil {
		t.Fatal(e)
	}
	requireDeleteErr(t, repo.DeleteDraft(ctx, "p", "a", 2), "draft_in_use")
	taskA.Status = task.StatusSucceeded
	if e := r.Save(ctx, taskA); e != nil {
		t.Fatal(e)
	}
	if e := repo.DeleteDraft(ctx, "p", "a", 2); e != nil {
		t.Fatal(e)
	}
	actual, e := repo.Get(ctx, "p")
	if e != nil || len(actual.Drafts) != 1 || actual.Drafts[0].ID != "b" || actual.Drafts[0].Version != 7 {
		t.Fatalf("other draft changed: %+v %v", actual, e)
	}
	if _, e = r.Get(ctx, "task"); e != nil {
		t.Fatal("completed task removed", e)
	}
	var count int
	if e = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_references WHERE project_id='p' AND draft_id='a'`).Scan(&count); e != nil || count != 0 {
		t.Fatalf("removed draft reference retained: %d %v", count, e)
	}
	if e = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_references WHERE project_id='p' AND draft_id='b'`).Scan(&count); e != nil || count != 1 {
		t.Fatalf("other draft reference removed: %d %v", count, e)
	}
	if _, e = r.GetMaterial(ctx, "shared"); e != nil {
		t.Fatal("shared material deleted", e)
	}
	requireDeleteErr(t, repo.Save(ctx, p), "resource_deleted")
	_, e = repo.Update(ctx, "p", func(p *project.Project) error { return p.AddDraft(project.Draft{ID: "a"}) })
	requireDeleteErr(t, e, "resource_deleted")
	late := task.Task{ID: "late", ProjectID: "p", DraftID: "a", Status: task.StatusQueued}
	requireDeleteErr(t, r.Save(ctx, late), "resource_deleted")
	_, _, e = r.CreateIfAbsent(ctx, late)
	requireDeleteErr(t, e, "resource_deleted")
	if e = repo.DeleteDraft(ctx, "p", "a", 2); !errors.Is(e, fault.ErrNotFound) {
		t.Fatal(e)
	}
	if e = repo.DeleteDraft(ctx, "p", "b", 7); e != nil {
		t.Fatal(e)
	}
	actual, e = repo.Get(ctx, "p")
	if e != nil || len(actual.Drafts) != 0 {
		t.Fatalf("last draft retained: %+v %v", actual, e)
	}
}
func TestConnectionDeletionCascadeAndLateTaskProtection(t *testing.T) {
	ctx := context.Background()
	r := deletionStore(t)
	for _, id := range []string{"c1", "c2"} {
		if e := r.SaveConnection(ctx, provider.Connection{ID: id, Name: id}); e != nil {
			t.Fatal(e)
		}
		if e := r.SaveModel(ctx, provider.Model{ID: id + ":m", ConnectionID: id}); e != nil {
			t.Fatal(e)
		}
	}
	running := task.Task{ID: "t", ModelID: "c1:m", Status: task.StatusQueued}
	if e := r.Save(ctx, running); e != nil {
		t.Fatal(e)
	}
	requireDeleteErr(t, r.DeleteConnection(ctx, "c1"), "connection_in_use")
	if _, e := r.GetModel(ctx, "c1:m"); e != nil {
		t.Fatal("failed deletion changed model", e)
	}
	running.Status = task.StatusSucceeded
	if e := r.Save(ctx, running); e != nil {
		t.Fatal(e)
	}
	if e := r.DeleteConnection(ctx, "c1"); e != nil {
		t.Fatal(e)
	}
	if _, e := r.GetConnection(ctx, "c1"); !errors.Is(e, fault.ErrNotFound) {
		t.Fatal(e)
	}
	if _, e := r.GetModel(ctx, "c1:m"); !errors.Is(e, fault.ErrNotFound) {
		t.Fatal(e)
	}
	if _, e := r.GetModel(ctx, "c2:m"); e != nil {
		t.Fatal(e)
	}
	if _, e := r.Get(ctx, "t"); e != nil {
		t.Fatal("history deleted", e)
	}
	requireDeleteErr(t, r.SaveConnection(ctx, provider.Connection{ID: "c1"}), "resource_deleted")
	requireDeleteErr(t, r.SaveModel(ctx, provider.Model{ID: "c1:new", ConnectionID: "c1"}), "resource_deleted")
	late := task.Task{ID: "late", ModelID: "c1:m", Status: task.StatusQueued}
	requireDeleteErr(t, r.Save(ctx, late), "resource_deleted")
	_, _, e := r.CreateIfAbsent(ctx, late)
	requireDeleteErr(t, e, "resource_deleted")
	if e = r.DeleteConnection(ctx, "c1"); e != nil {
		t.Fatal("retry must allow credential cleanup", e)
	}
}

func TestProjectDeletionAllDraftsActiveTaskAndLateWrites(t *testing.T) {
	ctx := context.Background()
	store := deletionStore(t)
	repo := NewProjectRepository(store)
	p := project.Project{ID: "delete-project", Name: "项目", Drafts: []project.Draft{{ID: "a", Version: 1}, {ID: "b", Version: 2}}}
	other := project.Project{ID: "keep-project", Name: "保留", Drafts: []project.Draft{{ID: "a"}}}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, other); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMaterial(ctx, material.Asset{ID: "keep-material", Kind: material.Scene}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"binding", "generation_snapshot"} {
		if _, err := store.db.ExecContext(ctx, `INSERT INTO material_references(project_id,draft_id,scene_id,material_id,source_kind,source_id) VALUES(?,?,?,?,?,?)`, p.ID, "a", "s", "keep-material", kind, kind); err != nil {
			t.Fatal(err)
		}
	}
	expected := project.DeletionExpectation{Name: p.Name, DraftVersions: map[string]int64{"a": 1, "b": 2}}
	stale := project.DeletionExpectation{Name: p.Name, DraftVersions: map[string]int64{"a": 1}}
	requireDeleteErr(t, repo.DeleteProjectChecked(ctx, p.ID, stale), "project_version_conflict")
	running := task.Task{ID: "project-task", ProjectID: p.ID, DraftID: "b", Status: task.StatusRunning}
	if err := store.Save(ctx, running); err != nil {
		t.Fatal(err)
	}
	requireDeleteErr(t, repo.DeleteProjectChecked(ctx, p.ID, expected), "project_in_use")
	if actual, err := repo.Get(ctx, p.ID); err != nil || len(actual.Drafts) != 2 {
		t.Fatal("failed deletion changed project", err)
	}
	running.Status = task.StatusSucceeded
	if err := store.Save(ctx, running); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteProjectChecked(ctx, p.ID, expected); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, p.ID); !errors.Is(err, fault.ErrNotFound) {
		t.Fatal("project retained", err)
	}
	if _, err := repo.Get(ctx, other.ID); err != nil {
		t.Fatal("other project removed", err)
	}
	if _, err := store.Get(ctx, running.ID); err != nil {
		t.Fatal("completed task removed", err)
	}
	if _, err := store.GetMaterial(ctx, "keep-material"); err != nil {
		t.Fatal("library asset removed", err)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_references WHERE project_id=? AND source_kind!='generation_snapshot'`, p.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("project references retained", err, count)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_references WHERE project_id=? AND source_kind='generation_snapshot'`, p.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("task snapshot removed", err, count)
	}
	requireDeleteErr(t, repo.Save(ctx, p), "resource_deleted")
	// A delayed creation of a previously unknown draft must not resurrect the project.
	requireDeleteErr(t, repo.Save(ctx, project.Project{ID: p.ID}), "resource_deleted")
	late := task.Task{ID: "late-new-draft", ProjectID: p.ID, DraftID: "new", Status: task.StatusQueued}
	requireDeleteErr(t, store.Save(ctx, late), "resource_deleted")
	_, _, err := store.CreateIfAbsent(ctx, late)
	requireDeleteErr(t, err, "resource_deleted")
	_, err = repo.Update(ctx, p.ID, func(p *project.Project) error { return p.AddDraft(project.Draft{ID: "new"}) })
	if !errors.Is(err, fault.ErrNotFound) {
		t.Fatal("late update succeeded", err)
	}
	if err = repo.DeleteProjectChecked(ctx, p.ID, expected); !errors.Is(err, fault.ErrNotFound) {
		t.Fatal("repeat deletion", err)
	}
	empty := project.Project{ID: "empty", Name: "空项目"}
	if err = repo.Save(ctx, empty); err != nil {
		t.Fatal(err)
	}
	if err = repo.DeleteProjectChecked(ctx, empty.ID, project.DeletionExpectation{Name: empty.Name, DraftVersions: map[string]int64{}}); err != nil {
		t.Fatal("empty project", err)
	}
}
