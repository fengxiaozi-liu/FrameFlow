package application

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	"os"
	"path/filepath"
	"testing"
)

func TestBindingResultIdempotentAndDeletedSceneNotRevived(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "result.png"), []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	scene := story.Scene{ID: "s", VisualPrompt: "cafe"}
	p := project.Project{ID: "p", Drafts: []project.Draft{{ID: "d", Story: story.Document{Scenes: []story.Scene{scene}}}}}
	if err = db.SaveProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	a := TaskResultApplier{Projects: sqlite.NewProjectRepository(db), Materials: sqlite.NewMaterialRepository(db), Directory: dir}
	item := task.Task{ID: "i", Kind: task.KindImage, ProjectID: "p", DraftID: "d", SceneID: "s", ResultURL: "/media/result.png", Snapshot: &task.GenerationSnapshot{ID: "snap", Path: "reference_image_then_video", DependencyHash: dependencyHash(p.EntityCatalog, scene, "reference_image_then_video", "")}}
	if err = a.Apply(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err = a.Apply(ctx, item); err != nil {
		t.Fatal(err)
	}
	got, _ := db.GetProject(ctx, "p")
	if len(got.Drafts[0].MediaTakes) != 1 || got.Drafts[0].MediaTakes[0].Stale {
		t.Fatal(got)
	}
	got.Drafts[0].Story.Scenes = nil
	if err = db.SaveProject(ctx, got); err != nil {
		t.Fatal(err)
	}
	item.ID = "late"
	if err = a.Apply(ctx, item); err != nil {
		t.Fatal(err)
	}
	got, _ = db.GetProject(ctx, "p")
	if len(got.Drafts[0].Story.Scenes) != 0 || len(got.Drafts[0].MediaTakes) != 1 {
		t.Fatal("deleted scene revived")
	}
}
