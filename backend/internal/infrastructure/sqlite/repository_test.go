package sqlite

import (
	"context"
	"database/sql"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	_ "modernc.org/sqlite"
	"path/filepath"
	"testing"
	"time"
)

func TestRepositoriesPersistAggregates(t *testing.T) {
	r, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	now := time.Now().UTC()
	taskValue := task.New("t1", task.KindVideo, task.Input{Prompt: "测试视频"}, now)
	if err = r.Save(context.Background(), taskValue); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get(context.Background(), "t1"); ok != nil {
		t.Fatal("task missing")
	}
	p, _ := project.New("p1", "Demo", now)
	if err = r.SaveProject(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.GetProject(context.Background(), "p1"); ok != nil {
		t.Fatal("project missing")
	}
	c := provider.Config{Code: "v1", Name: "Video", Capability: provider.Video, Status: provider.Disabled}
	if err = r.SaveProvider(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if items, err := r.ListProviders(context.Background(), provider.Video); err != nil || len(items) != 1 {
		t.Fatal("provider missing")
	}
	a, _ := material.New("m1", "Cover", material.Visual, now)
	if err = r.SaveMaterial(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if items, err := r.ListMaterials(context.Background(), material.Visual); err != nil || len(items) != 1 {
		t.Fatal("material missing")
	}
}

func TestBackupCanBeRestoredAndPassesIntegrityCheck(t *testing.T) {
	dir := t.TempDir()
	original, err := Open(context.Background(), filepath.Join(dir, "original.db"))
	if err != nil {
		t.Fatal(err)
	}
	value := task.New("backup-task", task.KindVideo, task.Input{Prompt: "备份视频"}, time.Now().UTC())
	if err = original.Save(context.Background(), value); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(dir, "backup", "frameflow.db")
	if err = original.Backup(context.Background(), backupPath); err != nil {
		t.Fatal(err)
	}
	_ = original.Close()

	restored, err := Open(context.Background(), backupPath)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if err = restored.VerifyIntegrity(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := restored.Get(context.Background(), value.ID); ok != nil {
		t.Fatal("restored database is missing the saved task")
	}
}

func TestTaskScopeAndLegacySchemaMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = legacy.Exec(`CREATE TABLE tasks (id TEXT PRIMARY KEY, payload TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	_ = legacy.Close()
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	first := task.New("t1", task.KindStory, task.Input{Prompt: "one"}, now)
	first.ProjectID, first.DraftID = "p1", "d1"
	second := task.New("t2", task.KindStory, task.Input{Prompt: "two"}, now)
	second.ProjectID, second.DraftID = "p2", "d2"
	if err = store.Save(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err = store.Save(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	items, err := store.ListByScope(context.Background(), task.Scope{ProjectID: "p1", DraftID: "d1"})
	if err != nil || len(items) != 1 || items[0].ID != "t1" {
		t.Fatalf("unexpected scoped tasks: %#v, %v", items, err)
	}
}

func TestProjectAtomicUpdatePreservesConcurrentResults(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "updates.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	p, _ := project.New("p", "demo", now)
	_ = p.AddDraft(project.Draft{ID: "d"})
	if err = store.SaveProject(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	repo := NewProjectRepository(store)
	errors := make(chan error, 2)
	for _, output := range []project.DraftOutput{
		{TaskID: "image", Kind: "image", URL: "/media/image.png"},
		{TaskID: "video", Kind: "video", URL: "/media/video.mp4"},
	} {
		output := output
		go func() {
			_, updateErr := repo.Update(context.Background(), "p", func(value *project.Project) error {
				return value.ApplyTaskResult("d", output, time.Now().UTC())
			})
			errors <- updateErr
		}()
	}
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	updated, err := repo.Get(context.Background(), "p")
	if err != nil || len(updated.Drafts[0].Outputs) != 2 {
		t.Fatalf("concurrent outputs lost: %#v, %v", updated, err)
	}
}
