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
	"strings"
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

func TestCompilerUsesExplicitFramesFromCurrentShotOnly(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	model := provider.MigrateLegacyModel(provider.Model{ID: "multi", RemoteID: "wan2.7-r2v", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Video}})
	binding := model.Operations[provider.Video]
	binding.InputMode = ""
	binding.InputModes = []string{"first_frame", "reference_images"}
	binding.LastFrame = true
	model.Operations[provider.Video] = binding
	model.RemoteID = "unknown-multimode"
	if err := db.SaveModel(ctx, model); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, id := range []string{"a", "b", "c", "outside"} {
		if err := os.WriteFile(filepath.Join(dir, id+".png"), []byte(id), 0600); err != nil {
			t.Fatal(err)
		}
		if err := db.SaveMaterial(ctx, material.Asset{ID: id, Kind: material.Scene, URL: "/media/" + id + ".png"}); err != nil {
			t.Fatal(err)
		}
	}
	p := project.Project{ID: "p", EntityCatalog: project.EntityCatalog{Entities: []project.EntityVersion{{ID: "scene", Kind: "location", Name: "Location", Revision: 1, References: []project.ReferenceImage{{MaterialID: "a"}, {MaterialID: "b"}, {MaterialID: "c"}}}}}}
	d := project.Draft{ID: "d", Version: 1}
	scene := story.Scene{ID: "s", VisualPrompt: "Move", EntityBindings: []story.EntityBinding{{ID: "binding", EntityID: "scene", EntityRevision: 1, Role: "primary"}}}
	compiler := GenerationCompiler{Models: db, Materials: sqlite.NewMaterialRepository(db), Directory: dir}
	version := int64(1)
	input := GenerationRequest{ExpectedVersion: &version, ModelID: model.ID, Path: "reference_video", VideoInputMode: "first_frame", FirstFrameMaterialID: "c", LastFrameMaterialID: "a", DurationSeconds: 4, Resolution: "720P", AspectRatio: "16:9"}
	first, err := compiler.Compile(ctx, p, d, scene, input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first.Prompt, "首帧属于Location") || !strings.Contains(first.Prompt, "尾帧属于Location") || strings.Contains(first.Prompt, "图1属于") {
		t.Fatal("frame role prompt is ambiguous", first.Prompt)
	}
	if first.FirstFrameMaterialID != "c" || first.LastFrameMaterialID != "a" || len(first.Entities[0].References) != 2 {
		t.Fatal(first)
	}
	input.FirstFrameMaterialID, input.LastFrameMaterialID = "a", "c"
	swapped, err := compiler.Compile(ctx, p, d, scene, input)
	if err != nil || swapped.InputHash == first.InputHash {
		t.Fatal("frame roles not frozen", err)
	}
	for _, id := range []string{"", "outside"} {
		input.FirstFrameMaterialID = id
		if _, err := compiler.Compile(ctx, p, d, scene, input); err == nil {
			t.Fatal("invalid frame accepted", id)
		}
	}
	input.VideoInputMode = "reference_images"
	input.FirstFrameMaterialID = "a"
	if _, err := compiler.Compile(ctx, p, d, scene, input); err == nil {
		t.Fatal("mixed input accepted")
	}
	input.FirstFrameMaterialID, input.LastFrameMaterialID = "", ""
	references, err := compiler.Compile(ctx, p, d, scene, input)
	if err != nil || len(references.Entities[0].References) != 3 {
		t.Fatal(references, err)
	}
	input.VideoInputMode = ""
	if _, err := compiler.Compile(ctx, p, d, scene, input); err == nil {
		t.Fatal("ambiguous mode accepted")
	}
}
