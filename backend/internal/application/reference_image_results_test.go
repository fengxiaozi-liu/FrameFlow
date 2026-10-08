package application

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestReferenceImageResultsRemainIndependentAndIdempotent(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p := project.Project{ID: "p", Drafts: []project.Draft{{ID: "d"}}}
	if err = db.SaveProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	file, err := os.Create(filepath.Join(dir, "result.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(file, image.NewRGBA(image.Rect(0, 0, 16, 9))); err != nil {
		t.Fatal(err)
	}
	file.Close()
	a := TaskResultApplier{Projects: sqlite.NewProjectRepository(db), Materials: sqlite.NewMaterialRepository(db), Directory: dir}
	for _, kind := range []string{"scene", "character"} {
		item := task.Task{ID: kind, Kind: task.KindImage, Target: "reference_" + kind, ProjectID: "p", DraftID: "d", Input: task.Input{MaterialName: "参考"}, ResultURL: "/media/result.png"}
		if err = a.Apply(ctx, item); err != nil {
			t.Fatal(err)
		}
		asset, err := a.Materials.Get(ctx, "result-"+kind)
		if err != nil || string(asset.Kind) != kind || asset.Media.Width != 16 {
			t.Fatal(asset, err)
		}
		asset.Name = "用户改名"
		if err = a.Materials.Save(ctx, asset); err != nil {
			t.Fatal(err)
		}
		if err = a.Apply(ctx, item); err != nil {
			t.Fatal(err)
		}
		asset, _ = a.Materials.Get(ctx, asset.ID)
		if asset.Name != "用户改名" {
			t.Fatal("replay overwrote material")
		}
	}
	got, _ := a.Projects.Get(ctx, "p")
	if len(got.Entities) != 0 || len(got.Drafts[0].Outputs) != 0 || len(got.Drafts[0].MediaTakes) != 0 {
		t.Fatal("generation changed draft/entity", got)
	}
	if err = a.Apply(ctx, task.Task{ID: "bad", Kind: task.KindImage, Target: "reference_scene", ProjectID: "p", DraftID: "d", ResultURL: "https://example.com/a.png"}); err == nil {
		t.Fatal("remote result accepted")
	}
}
