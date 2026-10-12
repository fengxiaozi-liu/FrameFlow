package processor

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/media"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
)

type imageRecordingClient struct {
	recordingClient
	request provider.ImageRequest
	calls   int
}

func (c *imageRecordingClient) GenerateImage(_ context.Context, _ provider.Connection, _ provider.Model, in provider.ImageRequest, _ provider.RequestOptions) (provider.ImageResult, error) {
	c.request = in
	c.calls++
	return provider.ImageResult{URL: "https://example.com/new.png"}, nil
}

func TestImageEditingExecutionUsesFrozenConfigurationAndOrderedBytes(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn := provider.Connection{ID: "c", Name: "test", Vendor: "bailian", BaseURL: "https://example.com"}
	if err := db.SaveConnection(ctx, conn); err != nil {
		t.Fatal(err)
	}
	frozen := provider.Model{ID: "m", ConnectionID: "c", RemoteID: "any-new-model", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Image}, Operations: map[provider.Capability]provider.OperationBinding{
		provider.Image: {Protocol: provider.ImageProtocol, Source: "manual", ImageModes: []string{provider.ImageToImage}, MinReferences: 1, MaxReferences: 3, MaxImageBytes: 10 << 20, Ratios: []string{"1:1"}, ImageSizes: map[string]string{"1:1": "1024*1024"}},
	}}
	current := frozen
	current.Operations = map[provider.Capability]provider.OperationBinding{provider.Image: {Protocol: provider.ImageProtocol, Source: "manual", ImageModes: []string{provider.TextToImage}, Ratios: []string{"1:1"}, ImageSizes: map[string]string{"1:1": "1024*1024"}}}
	if err := db.SaveModel(ctx, current); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6p9sAAAAASUVORK5CYII=")
	refs := []task.MediaReference{}
	for _, id := range []string{"second", "first"} {
		data := append(append([]byte{}, png...), []byte(id)...)
		hash := sha256.Sum256(data)
		if err := os.WriteFile(filepath.Join(dir, id+".png"), data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := db.SaveMaterial(ctx, material.Asset{ID: id, Kind: material.Scene, Name: id, URL: "/media/" + id + ".png"}); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, task.MediaReference{MaterialID: id, URL: "/media/" + id + ".png", ContentHash: hex.EncodeToString(hash[:])})
	}
	item := task.New("edit", task.KindImage, task.Input{Mode: provider.ImageToImage, Prompt: "edit image 1", AspectRatio: "1:1", ReferenceImages: refs}, time.Now())
	item.ModelID, item.ModelConfig = frozen.ID, &frozen
	client := &imageRecordingClient{}
	p := New(nil, db)
	p.ImageReferences = media.ImageReferences{Directory: dir}
	p.MediaDir, p.Models, p.ProviderConnections, p.ModelClient = dir, db, db, client
	if err := p.processModel(ctx, &item, func(int, task.Stage) {}); err != nil {
		t.Fatal(err)
	}
	if client.request.Mode != provider.ImageToImage || len(client.request.References) != 2 {
		t.Fatal(client.request)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(client.request.References[0].URL, "data:image/png;base64,"))
	if err != nil || !strings.HasSuffix(string(data), "second") {
		t.Fatal("input order changed", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "second.png")); err != nil {
		t.Fatal("original removed", err)
	}
	item.ResultURL = ""
	if err := os.WriteFile(filepath.Join(dir, "second.png"), png, 0600); err != nil {
		t.Fatal(err)
	}
	if err := p.processModel(ctx, &item, func(int, task.Stage) {}); err == nil || !strings.Contains(err.Error(), "changed since submission") {
		t.Fatal(err)
	}
	if client.calls != 1 {
		t.Fatal("changed input reached provider", client.calls)
	}
}
