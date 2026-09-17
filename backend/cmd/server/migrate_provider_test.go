package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/security"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
)

func TestLegacyProviderMigrationKeepsOriginalAndCredential(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := sqlite.Open(ctx, filepath.Join(dir, "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	vault, err := security.OpenVault(ctx, filepath.Join(dir, "secret"))
	if err != nil {
		t.Fatal(err)
	}
	legacy := provider.Config{Code: "old-image", Name: "Old image", Vendor: "bailian", Capability: provider.Image, Model: "qwen-image-2.0", BaseURL: "https://dashscope.aliyuncs.com", Enabled: true, Status: provider.Healthy, CredentialSet: true}
	if err := store.SaveProvider(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	if err := vault.Put(ctx, legacy.Code, "original-key"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := migrateProviders(ctx, store, vault); err != nil {
			t.Fatal(err)
		}
	}
	connection, err := store.GetConnection(ctx, "legacy-old-image")
	if err != nil || !connection.CredentialSet || connection.BaseURL != legacy.BaseURL {
		t.Fatal(connection, err)
	}
	key, err := vault.Get(ctx, "connection:"+connection.ID)
	if err != nil || key != "original-key" {
		t.Fatal(err)
	}
	model, err := store.GetModel(ctx, provider.ModelID(connection.ID, legacy.Model))
	if err != nil || model.Enabled || model.Capabilities[0] != provider.Image {
		t.Fatal(model, err)
	}
	if _, err := store.GetProvider(ctx, legacy.Code); err != nil {
		t.Fatal("legacy row removed", err)
	}
}
