package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCandidateAndSnapshotSurviveRepositoryReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidates.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	p, _ := project.New("p", "demo", now)
	d := project.Draft{ID: "d", Story: story.Document{Body: "edited", Scenes: []story.Scene{{ID: "s", Order: 1, Title: "scene"}}}}
	if err = d.AddCandidate(story.Candidate{ID: "candidate", Target: story.CandidateStoryboard, SourceVersion: 0, Status: "preview"}); err != nil {
		t.Fatal(err)
	}
	if err = d.CaptureStoryboardSnapshot("snapshot", now); err != nil {
		t.Fatal(err)
	}
	if err = d.AddBinding(project.SceneBinding{SceneID: "s", MaterialID: "m", Usage: "first_frame"}); err != nil {
		t.Fatal(err)
	}
	if err = d.AddVideoVersion(project.VideoVersion{ID: "v", SceneID: "s", TaskID: "t", LocalMediaPath: "clip.mp4"}); err != nil {
		t.Fatal(err)
	}
	if err = d.SelectVideoVersion("s", "v"); err != nil {
		t.Fatal(err)
	}
	if err = d.AddComposition(project.Composition{ID: "film", TaskID: "compose", ClipVersionIDs: []string{"v"}}); err != nil {
		t.Fatal(err)
	}
	if err = p.AddDraft(d); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveProject(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	store, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	loaded, err := store.GetProject(context.Background(), "p")
	if err != nil || len(loaded.Drafts) != 1 || len(loaded.Drafts[0].Candidates) != 1 || len(loaded.Drafts[0].StoryboardSnapshots) != 1 || loaded.Drafts[0].Story.Body != "edited" || len(loaded.Drafts[0].Bindings) != 1 || len(loaded.Drafts[0].VideoVersions) != 1 || loaded.Drafts[0].SelectedVersions["s"] != "v" || len(loaded.Drafts[0].Compositions) != 1 {
		t.Fatalf("candidate or snapshot lost: %+v, %v", loaded, err)
	}
}

func TestLegacyMaterialMigrationIsIdempotentAndKeepsIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-materials.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE materials(id TEXT PRIMARY KEY,kind TEXT NOT NULL,payload TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/legacy/materials.json")
	if err != nil {
		t.Fatal(err)
	}
	var old []material.Asset
	if err = json.Unmarshal(raw, &old); err != nil {
		t.Fatal(err)
	}
	for _, item := range old {
		payload, _ := json.Marshal(item)
		if _, err = db.Exec(`INSERT INTO materials(id,kind,payload) VALUES(?,?,?)`, item.ID, item.Kind, string(payload)); err != nil {
			t.Fatal(err)
		}
	}
	_ = db.Close()
	for round := 0; round < 2; round++ {
		store, openErr := Open(context.Background(), path)
		if openErr != nil {
			t.Fatal(openErr)
		}
		items, listErr := store.ListMaterials(context.Background(), material.Scene)
		if listErr != nil || len(items) != 2 {
			t.Fatalf("scene migration: %+v, %v", items, listErr)
		}
		frame, getErr := store.GetMaterial(context.Background(), "m-frame")
		if getErr != nil || frame.ID != "m-frame" || frame.URL != "/media/m-frame.png" || frame.LegacyKind != material.Frame || len(frame.Tags) != 1 || frame.Tags[0] != "原首尾帧" {
			t.Fatalf("legacy frame changed meaning: %+v, %v", frame, getErr)
		}
		_ = store.Close()
	}
}

func TestLegacyMaterialMigrationRollsBackOnBadPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollback.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE materials(id TEXT PRIMARY KEY,kind TEXT NOT NULL,payload TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	old := material.Asset{ID: "old", Name: "Old", Kind: material.Frame, URL: "/media/old.png"}
	payload, _ := json.Marshal(old)
	if _, err = db.Exec(`INSERT INTO materials(id,kind,payload) VALUES('a-old','frame',?),('z-bad','frame','{invalid')`, string(payload)); err != nil {
		t.Fatal(err)
	}
	if err = migrateStoryWorkspace(context.Background(), db); err == nil {
		t.Fatal("invalid legacy payload should fail migration")
	}
	var kind, stored string
	if err = db.QueryRow(`SELECT kind,payload FROM materials WHERE id='a-old'`).Scan(&kind, &stored); err != nil {
		t.Fatal(err)
	}
	if kind != "frame" || stored != string(payload) {
		t.Fatalf("migration partially changed material: %s %s", kind, stored)
	}
	var migrations int
	if err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if migrations != 0 {
		t.Fatal("migration metadata survived rollback")
	}
}

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
