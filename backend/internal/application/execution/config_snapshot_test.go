package processor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomFirstFrameUsesFrozenProtocolAfterConfigurationChanges(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, e := sqlite.Open(ctx, filepath.Join(dir, "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	conn := provider.Connection{ID: "c", Name: "c", Vendor: "custom", BaseURL: "https://example.com"}
	if e = store.SaveConnection(ctx, conn); e != nil {
		t.Fatal(e)
	}
	model := provider.MigrateLegacyModel(provider.Model{ID: "custom", ConnectionID: "c", RemoteID: "wan2.7-i2v", Capabilities: []provider.Capability{provider.Video}, Enabled: true, Supported: true})
	model.RemoteID = "opaque-video-id"
	if e = store.SaveModel(ctx, model); e != nil {
		t.Fatal(e)
	}
	data := []byte("frame")
	if e = os.WriteFile(filepath.Join(dir, "frame.png"), data, 0600); e != nil {
		t.Fatal(e)
	}
	hash := sha256.Sum256(data)
	snap, e := task.FreezeSnapshot(task.GenerationSnapshot{ModelID: model.ID, ModelConfig: &model, ProjectID: "p", DraftID: "d", SceneID: "s", CapabilityVersion: "config.1", Path: "reference_video", Prompt: "move", AspectRatio: "16:9", DurationSeconds: 4, Resolution: "720P", Entities: []task.SnapshotEntity{{EntityID: "person", References: []task.MediaReference{{MaterialID: "frame", ContentHash: hex.EncodeToString(hash[:]), URL: "/media/frame.png"}}}}})
	if e != nil {
		t.Fatal(e)
	}
	item := task.Task{Kind: task.KindVideo, ModelID: model.ID, ModelConfig: snap.ModelConfig, Snapshot: &snap}
	other := provider.MigrateLegacyModel(provider.Model{RemoteID: "wan2.7-r2v", Capabilities: []provider.Capability{provider.Video}})
	if _, e = store.ConfigureOperations(ctx, model.ID, model.Revision, other.Operations, nil); e != nil {
		t.Fatal(e)
	}
	client := &recordingClient{}
	p := New(nil, store)
	p.Task = nil
	p.Models = store
	p.ProviderConnections = store
	p.ModelClient = client
	p.MediaDir = dir
	if e = p.processModel(ctx, &item, func(int, task.Stage) {}); e != nil {
		t.Fatal(e)
	}
	if len(client.request.References) != 0 || !strings.HasPrefix(client.request.SourceImageURL, "data:image/") || client.request.Model != "opaque-video-id" {
		t.Fatal(client.request)
	}
}

func TestFrozenFrameRolesSurviveModelChangeAndRetry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := sqlite.Open(ctx, filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	conn := provider.Connection{ID: "c", Name: "test", Vendor: "custom", BaseURL: "https://example.com"}
	if err := store.SaveConnection(ctx, conn); err != nil {
		t.Fatal(err)
	}
	model := provider.MigrateLegacyModel(provider.Model{ID: "multi", ConnectionID: "c", RemoteID: "wan2.7-r2v", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Video}})
	binding := model.Operations[provider.Video]
	binding.InputMode = ""
	binding.InputModes = []string{"first_frame", "reference_images"}
	binding.LastFrame = true
	model.Operations[provider.Video] = binding
	if err := store.SaveModel(ctx, model); err != nil {
		t.Fatal(err)
	}
	snap := task.GenerationSnapshot{ModelID: model.ID, ModelConfig: &model, ProjectID: "p", DraftID: "d", SceneID: "s", CapabilityVersion: "v1", Path: "reference_video", VideoInputMode: "first_frame", FirstFrameMaterialID: "b", LastFrameMaterialID: "a", DurationSeconds: 4, Resolution: "720P", AspectRatio: "16:9"}
	for _, id := range []string{"a", "b"} {
		data := []byte(id)
		if err := os.WriteFile(filepath.Join(dir, id+".png"), data, 0600); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		snap.Entities = append(snap.Entities, task.SnapshotEntity{EntityID: id, References: []task.MediaReference{{MaterialID: id, URL: "/media/" + id + ".png", ContentHash: hex.EncodeToString(hash[:])}}})
	}
	snap, err = task.FreezeSnapshot(snap)
	if err != nil {
		t.Fatal(err)
	}
	binding.InputModes = []string{"reference_images"}
	binding.LastFrame = false
	if _, err := store.ConfigureOperations(ctx, model.ID, model.Revision, map[provider.Capability]provider.OperationBinding{provider.Video: binding}, nil); err != nil {
		t.Fatal(err)
	}
	client := &recordingClient{}
	p := New(nil, store)
	p.Task = nil
	p.Models = store
	p.ProviderConnections = store
	p.ModelClient = client
	p.MediaDir = dir
	for i := 0; i < 2; i++ {
		item := task.Task{Kind: task.KindVideo, ModelID: model.ID, ModelConfig: snap.ModelConfig, Snapshot: &snap}
		if err := p.processModel(ctx, &item, func(int, task.Stage) {}); err != nil {
			t.Fatal(err)
		}
		if client.request.InputMode != "first_frame" || len(client.request.References) != 0 || !strings.HasSuffix(client.request.SourceImageURL, "Yg==") || !strings.HasSuffix(client.request.LastFrameURL, "YQ==") {
			t.Fatal(client.request)
		}
	}
}
