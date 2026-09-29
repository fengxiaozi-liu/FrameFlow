package application

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	"os"
	"path/filepath"
	"testing"
)

func TestCompilerFixesOrderedContentAndRejectsLimit(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.SaveModel(ctx, provider.Model{ID: "m", RemoteID: "qwen-image-2.0-pro", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Image}}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, id := range []string{"a", "b", "c", "d"} {
		if err = os.WriteFile(filepath.Join(dir, id+".png"), []byte(id), 0600); err != nil {
			t.Fatal(err)
		}
		if err = db.SaveMaterial(ctx, material.Asset{ID: id, Kind: material.Scene, URL: "/media/" + id + ".png"}); err != nil {
			t.Fatal(err)
		}
	}
	p := project.Project{ID: "p", EntityCatalog: project.EntityCatalog{Entities: []project.EntityVersion{{ID: "c", Kind: "character", Name: "Ming", Revision: 1, Description: "blue coat", References: []project.ReferenceImage{{MaterialID: "b"}, {MaterialID: "a"}}}}}}
	d := project.Draft{ID: "d", Version: 2}
	scene := story.Scene{ID: "s", VisualPrompt: "walk", EntityBindings: []story.EntityBinding{{ID: "b", EntityID: "c", EntityRevision: 1, Role: "cast"}}}
	v := int64(2)
	compiler := GenerationCompiler{Models: db, Materials: sqlite.NewMaterialRepository(db), Directory: dir}
	in := GenerationRequest{ExpectedVersion: &v, ModelID: "m", Path: "reference_image_then_video"}
	first, err := compiler.Compile(ctx, p, d, scene, in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Entities[0].References[0].MaterialID != "b" {
		t.Fatal("order lost")
	}
	if err = os.WriteFile(filepath.Join(dir, "b.png"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := compiler.Compile(ctx, p, d, scene, in)
	if err != nil || first.InputHash == second.InputHash {
		t.Fatal("content not fingerprinted", err)
	}
	p.Entities[0].References = append(p.Entities[0].References, project.ReferenceImage{MaterialID: "c"}, project.ReferenceImage{MaterialID: "d"})
	if _, err = compiler.Compile(ctx, p, d, scene, in); err == nil {
		t.Fatal("limit silently ignored")
	}
}
