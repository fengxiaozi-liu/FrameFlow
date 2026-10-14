package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
)

func TestConfiguredRatioSurvivesPersistenceAndGenerationSnapshots(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "frame.png"), []byte("frame"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveMaterial(ctx, material.Asset{ID: "frame", Kind: material.Scene, URL: "/media/frame.png"}); err != nil {
		t.Fatal(err)
	}
	b := provider.OperationBinding{Protocol: provider.CustomProtocol, Source: "manual", InputMode: "first_frame", Ratios: []string{"adaptive", "VendorWide"}, Durations: []int{5}, Resolutions: []string{"720P"}, MaxReferences: 1, MaxImageBytes: 1024,
		Custom: &provider.CustomInterface{Path: "/videos", Mode: "sync", AuthMode: "none", BodyTemplate: `{"model":"{{model}}","prompt":"{{prompt}}","first":"{{first_frame}}","ratio":"{{ratio}}"}`, ResultPointer: "/url"}}
	m := provider.Model{ID: "opaque", RemoteID: "unfamiliar/model", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Video}, Operations: map[provider.Capability]provider.OperationBinding{provider.Video: b}}
	if err := db.SaveModel(ctx, m); err != nil {
		t.Fatal(err)
	}
	p := project.Project{ID: "p", EntityCatalog: project.EntityCatalog{Entities: []project.EntityVersion{{ID: "location", Kind: "location", Name: "Location", Revision: 1, References: []project.ReferenceImage{{MaterialID: "frame"}}}}}}
	d := project.Draft{ID: "d", Version: 1, Bindings: []project.SceneBinding{{SceneID: "s", Usage: "first_frame", MaterialID: "frame"}}}
	scene := story.Scene{ID: "s", VisualPrompt: "Move", DurationSeconds: 5, EntityBindings: []story.EntityBinding{{ID: "binding", EntityID: "location", EntityRevision: 1, Role: "primary"}}}
	compiler := GenerationCompiler{Models: db, Materials: sqlite.NewMaterialRepository(db), Directory: dir}
	v := int64(1)
	for _, path := range []string{"reference_video", "first_frame_video"} {
		input := GenerationRequest{ExpectedVersion: &v, ModelID: m.ID, Path: path, AspectRatio: "adaptive", DurationSeconds: 5, Resolution: "720P"}
		if path == "reference_video" {
			input.VideoInputMode = "first_frame"
			input.FirstFrameMaterialID = "frame"
		}
		snap, err := compiler.Compile(ctx, p, d, scene, input)
		if err != nil || snap.AspectRatio != "adaptive" {
			t.Fatal(path, snap.AspectRatio, err)
		}
		input.AspectRatio = "16:9"
		if _, err := compiler.Compile(ctx, p, d, scene, input); err == nil {
			t.Fatal("accepted undeclared ratio", path)
		}
		b.Ratios = []string{"VendorWide"}
		m.Operations[provider.Video] = b
		if err := db.SaveModel(ctx, m); err != nil {
			t.Fatal(err)
		}
		input.AspectRatio = "adaptive"
		if _, err := compiler.Compile(ctx, p, d, scene, input); err == nil {
			t.Fatal("removed ratio accepted", path)
		}
		if snap.AspectRatio != "adaptive" || snap.ModelConfig.Operations[provider.Video].Ratios[0] != "adaptive" {
			t.Fatal("frozen configuration changed")
		}
		b.Ratios = []string{"adaptive", "VendorWide"}
		m.Operations[provider.Video] = b
		if err := db.SaveModel(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
}
