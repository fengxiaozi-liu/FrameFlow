package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/media"
)

func TestImageEditingRequiresConfigurationAndFreezesOrderedReferences(t *testing.T) {
	s, done := testServer(t)
	defer done()
	s.TaskService.Enqueuer = nil
	s.TaskService.UploadDir = t.TempDir()
	s.TaskService.ImageReferences = media.ImageReferences{Directory: s.TaskService.UploadDir}
	ctx := context.Background()
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6p9sAAAAASUVORK5CYII=")
	for _, id := range []string{"a", "b", "other", "audio", "invalid"} {
		asset := material.Asset{ID: id, Name: id, Kind: material.Scene, URL: "/media/" + id + ".png", ProjectID: "test-project", ReferenceOnly: true}
		if id == "other" {
			asset.ProjectID = "another-project"
		}
		if id == "audio" {
			asset.Kind = material.Voice
		}
		data := png
		if id == "invalid" {
			data = []byte("not an image")
		}
		if err := os.WriteFile(filepath.Join(s.TaskService.UploadDir, id+".png"), data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := s.TaskService.Materials.Save(ctx, asset); err != nil {
			t.Fatal(err)
		}
	}
	r := testRouter(s)
	url := "/api/projects/test-project/drafts/test-draft/reference-image-tasks"
	input := map[string]any{"idempotency_key": "edit", "kind": "scene", "name": "晚霞", "prompt": "保留图1的构图，采用图2的光线", "model_id": "test-connection:image", "aspect_ratio": "1:1", "mode": "image_to_image", "reference_material_ids": []string{"b", "a"}}
	body := func() string { b, _ := json.Marshal(input); return string(b) }
	model, _ := s.Providers.Catalog.Models.GetModel(ctx, "test-connection:image")
	b := model.Operations[provider.Image]
	b.ImageModes = []string{provider.TextToImage}
	model.Operations[provider.Image] = b
	if err := s.Providers.Catalog.Models.SaveModel(ctx, model); err != nil {
		t.Fatal(err)
	}
	if w := request(t, r, "POST", url, body()); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	b.ImageModes = []string{provider.TextToImage, provider.ImageToImage}
	model.Operations[provider.Image] = b
	if err := s.Providers.Catalog.Models.SaveModel(ctx, model); err != nil {
		t.Fatal(err)
	}
	for _, refs := range [][]string{{}, {"a", "a"}, {"a", "b", "other", "audio"}, {"missing"}, {"other"}, {"audio"}, {"invalid"}} {
		input["reference_material_ids"] = refs
		if w := request(t, r, "POST", url, body()); w.Code != 400 {
			t.Fatal(refs, w.Code, w.Body.String())
		}
	}
	input["reference_material_ids"] = []string{"b", "a"}
	b.MaxImageBytes = 1
	model.Operations[provider.Image] = b
	_ = s.Providers.Catalog.Models.SaveModel(ctx, model)
	if w := request(t, r, "POST", url, body()); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	b.MaxImageBytes = 10 << 20
	model.Operations[provider.Image] = b
	_ = s.Providers.Catalog.Models.SaveModel(ctx, model)
	w := request(t, r, "POST", url, body())
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var saved task.Task
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Input.Mode != provider.ImageToImage || len(saved.Input.ReferenceImages) != 2 || saved.Input.ReferenceImages[0].MaterialID != "b" || saved.Input.ReferenceImages[1].ContentHash == "" || saved.ModelConfig == nil {
		t.Fatal(saved)
	}
	asset, _ := s.TaskService.Materials.Get(ctx, "a")
	if !asset.ReferenceOnly {
		t.Fatal("input was published automatically")
	}
	if w := request(t, r, "DELETE", "/api/materials/a", ""); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	// Lost-response replay uses the stored request even after model configuration changes.
	b.ImageModes = []string{provider.TextToImage}
	model.Operations[provider.Image] = b
	_ = s.Providers.Catalog.Models.SaveModel(ctx, model)
	if w := request(t, r, "POST", url, body()); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(t, r, "POST", "/api/tasks/"+saved.ID+"/cancel", "{}"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(t, r, "POST", "/api/tasks/"+saved.ID+"/retry", "{}")
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var retried task.Task
	if err := json.Unmarshal(w.Body.Bytes(), &retried); err != nil {
		t.Fatal(err)
	}
	if retried.Input.ReferenceImages[0].MaterialID != "b" || !retried.ModelConfig.Operations[provider.Image].SupportsImageMode(provider.ImageToImage) {
		t.Fatal(retried)
	}
	input["reference_material_ids"] = []string{"a", "b"}
	if w := request(t, r, "POST", url, body()); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	// Callers cannot bypass ownership checks through the generic task endpoint.
	if w := request(t, r, "POST", "/api/tasks", `{"kind":"image","prompt":"edit","reference_images":[{"material_id":"other","url":"/media/other.png"}]}`); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
}
