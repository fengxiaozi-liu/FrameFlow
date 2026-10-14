package sqlite

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"testing"
	"time"
)

func TestRunDeletionReleasesOnlyUnsharedSnapshotReferences(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	materials := NewMaterialRepository(db)
	for _, id := range []string{"shared", "exclusive"} {
		if err = materials.Save(ctx, material.Asset{ID: id, Kind: material.Scene, URL: "/media/" + id + ".png"}); err != nil {
			t.Fatal(err)
		}
	}
	// A snapshot is content addressed, so identical references can be used by separate runs.
	shared := task.GenerationSnapshot{ID: "snap-shared"}
	exclusive := task.GenerationSnapshot{ID: "snap-exclusive"}
	p := project.Project{ID: "p", Drafts: []project.Draft{{ID: "d", GenerationRuns: []task.GenerationRun{{ID: "r", SceneID: "s", State: "failed", TaskIDs: []string{"r-0", "r-1"}}, {ID: "keep", SceneID: "s", State: "completed", TaskIDs: []string{"keep-0"}}}}}}
	repo := NewProjectRepository(db)
	if err = repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}
	for _, child := range []task.Task{{ID: "r-0", RunID: "r", Snapshot: &shared}, {ID: "r-1", RunID: "r", Snapshot: &exclusive}, {ID: "keep-0", RunID: "keep", Snapshot: &shared}} {
		child.ProjectID = "p"
		child.DraftID = "d"
		child.SceneID = "s"
		child.Status = task.StatusFailed
		if err = db.Save(ctx, child); err != nil {
			t.Fatal(err)
		}
	}
	for _, ref := range []struct{ source, material string }{{"snap-shared", "shared"}, {"snap-exclusive", "exclusive"}, {"r-1", "exclusive"}} {
		if _, err = db.db.ExecContext(ctx, `INSERT INTO material_references(project_id,draft_id,scene_id,material_id,source_kind,source_id) VALUES('p','d','s',?,'generation_snapshot',?)`, ref.material, ref.source); err != nil {
			t.Fatal(err)
		}
	}
	if err = repo.DeleteFailedGenerationRun(ctx, "p", "d", "s", "r", time.Now()); err != nil {
		t.Fatal(err)
	}
	var sharedCount, exclusiveCount int
	if err = db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_references WHERE material_id='shared'`).Scan(&sharedCount); err != nil {
		t.Fatal(err)
	}
	if err = db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_references WHERE material_id='exclusive'`).Scan(&exclusiveCount); err != nil {
		t.Fatal(err)
	}
	if sharedCount != 1 || exclusiveCount != 0 {
		t.Fatalf("references shared=%d exclusive=%d", sharedCount, exclusiveCount)
	}
	if _, err = materials.Get(ctx, "exclusive"); err != nil {
		t.Fatal("material removed", err)
	}
}
