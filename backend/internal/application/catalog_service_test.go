package application

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/security"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	"github.com/gin-gonic/gin"
)

type testDiscovery struct{ models []provider.DiscoveredModel }

func (d testDiscovery) Discover(context.Context, provider.Connection, string) ([]provider.DiscoveredModel, error) {
	return d.models, nil
}

func TestCatalogSyncPreservesManualAndRejectsUnknown(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := sqlite.Open(ctx, filepath.Join(dir, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	vault, err := security.OpenVault(ctx, filepath.Join(dir, "vault"))
	if err != nil {
		t.Fatal(err)
	}
	s := CatalogService{Connections: store, Models: store, Vault: vault, Discovery: testDiscovery{models: []provider.DiscoveredModel{{ID: "qwen-plus", Name: "Cloud name", Capabilities: []provider.Capability{provider.Story}}, {ID: "unknown-model", Name: "Unknown"}}}}
	router := gin.New()
	router.POST("/connections", s.SaveConnection)
	router.POST("/connections/:id/models", s.AddModel)
	router.POST("/connections/:id/models/sync", s.SyncModels)
	router.PATCH("/connections/:id/models/:model", s.UpdateModel)
	router.DELETE("/connections/:id/models/:model", s.DeleteModel)
	request := func(method, path, body string, want int) []byte {
		t.Helper()
		out := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(out, req)
		if out.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, out.Code, out.Body.String())
		}
		return out.Body.Bytes()
	}
	request("POST", "/connections", `{"id":"invalid","name":"Invalid","vendor":"bailian","base_url":"https://dashscope.aliyuncs.com"}`, 400)
	request("POST", "/connections", `{"id":"beijing","name":"Beijing","vendor":"bailian","base_url":"https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions","api_key":"secret"}`, 201)
	request("POST", "/connections/beijing/models", `{"model_id":"qwen-plus","name":"My Qwen","capabilities":["story"]}`, 201)
	request("POST", "/connections/beijing/models/sync", `{}`, 200)
	manual, err := store.GetModel(ctx, "beijing:qwen-plus")
	if err != nil || manual.Name != "My Qwen" || manual.Source != "manual" {
		t.Fatalf("manual model overwritten: %+v %v", manual, err)
	}
	unknown, err := store.GetModel(ctx, "beijing:unknown-model")
	if err != nil || unknown.Supported || unknown.Enabled {
		t.Fatalf("unknown model enabled: %+v %v", unknown, err)
	}
	s.Discovery = testDiscovery{models: []provider.DiscoveredModel{{ID: "qwen-plus", Name: "Cloud name", Capabilities: []provider.Capability{provider.Story}}, {ID: "unknown-model", Name: "Unknown"}, {ID: "qwen-image", Name: "Qwen Image", Capabilities: []provider.Capability{provider.Image}}}}
	router.POST("/connections/:id/models/sync-updated", s.SyncModels)
	stale := provider.Model{ID: "beijing:qwen-image", ConnectionID: "beijing", RemoteID: "qwen-image", Name: "Qwen Image", Source: "discovered"}
	if err := store.SaveModel(ctx, stale); err != nil {
		t.Fatal(err)
	}
	request("POST", "/connections/beijing/models/sync-updated", `{}`, 200)
	refreshed, err := store.GetModel(ctx, stale.ID)
	if err != nil || !refreshed.Supports(provider.Image) {
		t.Fatalf("newly recognized model not enabled: %+v %v", refreshed, err)
	}
	request("PATCH", "/connections/beijing/models/beijing:unknown-model", `{"enabled":true}`, 400)
	request("PATCH", "/connections/beijing/models/beijing:unknown-model", `{"capabilities":["image"],"enabled":true}`, 200)
	configured, _ := store.GetModel(ctx, "beijing:unknown-model")
	if !configured.Supports(provider.Image) {
		t.Fatal("capability selection not persisted")
	}
	request("POST", "/connections/beijing/models/sync", `{}`, 200)
	configured, _ = store.GetModel(ctx, "beijing:unknown-model")
	if !configured.Supports(provider.Image) {
		t.Fatal("sync removed manually selected capabilities")
	}
	request("PATCH", "/connections/beijing/models/beijing:unknown-model", `{"capabilities":["invalid"]}`, 400)
	request("PATCH", "/connections/beijing/models/beijing:qwen-plus", `{"default":true}`, 200)
	manual, _ = store.GetModel(ctx, "beijing:qwen-plus")
	if !manual.Default {
		t.Fatal("default model not saved")
	}
	s.Tasks = store
	created := task.New("task-1", task.KindStory, task.Input{Prompt: "test"}, time.Now())
	created.ModelID = manual.ID
	if err := store.Save(ctx, created); err != nil {
		t.Fatal(err)
	}
	router.DELETE("/protected/:id/models/:model", s.DeleteModel)
	request("DELETE", "/protected/beijing/models/beijing:qwen-plus", "", 409)
	var saved provider.Connection
	if err := json.Unmarshal(request("POST", "/connections", `{"id":"other","name":"Other","vendor":"bailian","base_url":"https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"}`, 201), &saved); err != nil || saved.CredentialSet {
		t.Fatal(saved, err)
	}
}
