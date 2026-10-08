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

func TestReferenceVideoPreservesAllFrozenReferences(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := sqlite.Open(ctx, filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	connection := provider.Connection{ID: "test", Name: "Test", Vendor: "bailian", BaseURL: "https://example.com"}
	model := provider.Model{ID: "video", ConnectionID: "test", RemoteID: "wan2.7-r2v", Supported: true, Enabled: true, Capabilities: []provider.Capability{provider.Video}}
	if err := store.SaveConnection(ctx, connection); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveModel(ctx, model); err != nil {
		t.Fatal(err)
	}
	snap := task.GenerationSnapshot{ProjectID: "p", DraftID: "d", SceneID: "s", ModelID: model.ID, CapabilityVersion: "bindings.r2v.v1", Path: "reference_video", Prompt: "frozen scene", DurationSeconds: 6, Resolution: "720P", AspectRatio: "16:9"}
	for _, name := range []string{"scene", "person"} {
		data := []byte(name)
		if err := os.WriteFile(filepath.Join(dir, name+".png"), data, 0600); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		snap.Entities = append(snap.Entities, task.SnapshotEntity{EntityID: name, References: []task.MediaReference{{MaterialID: name, ContentHash: hex.EncodeToString(hash[:]), URL: "/media/" + name + ".png"}}})
	}
	snap, err = task.FreezeSnapshot(snap)
	if err != nil {
		t.Fatal(err)
	}
	client := &recordingClient{}
	p := New(nil, store)
	p.Task = nil
	p.Models, p.ProviderConnections, p.ModelClient, p.MediaDir = store, store, client, dir
	item := task.Task{Kind: task.KindVideo, ModelID: model.ID, Snapshot: &snap}
	if err := p.processModel(ctx, &item, func(int, task.Stage) {}); err != nil {
		t.Fatal(err)
	}
	if len(client.request.References) != 2 || client.request.SourceImageURL != "" || client.request.Prompt != "frozen scene" {
		t.Fatal(client.request)
	}
	if !strings.HasPrefix(client.request.References[1].URL, "data:image/png;base64,") {
		t.Fatal("local reference not delivered")
	}
	if err := os.WriteFile(filepath.Join(dir, "person.png"), []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := p.processModel(ctx, &item, func(int, task.Stage) {}); err == nil {
		t.Fatal("mutated reference accepted")
	}
}
