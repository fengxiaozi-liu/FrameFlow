package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

func TestIdentitySyncPreservesConfiguredModelAndRejectsStaleConnection(t *testing.T) {
	r, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	c := provider.Connection{ID: "c", Name: "Custom", Vendor: "custom", BaseURL: "https://example.com"}
	if err := r.SaveConnection(ctx, c); err != nil {
		t.Fatal(err)
	}
	c, _ = r.GetConnection(ctx, "c")
	m := provider.Model{ID: "c:video", ConnectionID: "c", RemoteID: "video", Name: "My Video", NameConfigured: true, Source: "discovered", Capabilities: []provider.Capability{provider.Video}, CapabilitiesConfigured: true, Enabled: true, Revision: 1}
	if err := r.SaveModel(ctx, m); err != nil {
		t.Fatal(err)
	}
	entries := []provider.DiscoveredModel{{ID: "video", Name: "Changed", Capabilities: []provider.Capability{provider.Image}}, {ID: "new", Name: "New"}}
	if count, err := r.SyncModelIdentities(ctx, c, entries, time.Now()); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	got, _ := r.GetModel(ctx, m.ID)
	if got.Name != "My Video" || !got.Enabled || got.Capabilities[0] != provider.Video {
		t.Fatal(got)
	}
	newModel, _ := r.GetModel(ctx, "c:new")
	if len(newModel.Capabilities) != 0 || newModel.Enabled {
		t.Fatal("sync granted capability", newModel)
	}
	if err := r.SaveConnection(ctx, c); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SyncModelIdentities(ctx, c, entries, time.Now()); err == nil {
		t.Fatal("stale sync accepted")
	}
}
