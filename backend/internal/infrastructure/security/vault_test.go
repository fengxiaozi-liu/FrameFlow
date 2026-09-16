package security

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCancelledVaultOperationsDoNotMutateCredentials(t *testing.T) {
	v, err := OpenVault(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Put(context.Background(), "key", "original"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := v.Put(ctx, "key", "changed"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := v.Delete(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := v.Get(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := v.Has(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if got, err := v.Get(context.Background(), "key"); err != nil || got != "original" {
		t.Fatal(got, err)
	}
}

func TestVaultEncryptsCredentials(t *testing.T) {
	dir := t.TempDir()
	v, e := OpenVault(context.Background(), dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = v.Put(context.Background(), "provider-a", "sk-secret"); e != nil {
		t.Fatal(e)
	}
	value, ok := v.Get(context.Background(), "provider-a")
	if ok != nil || value != "sk-secret" {
		t.Fatal(value)
	}
	raw, e := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if e != nil || strings.Contains(string(raw), "sk-secret") {
		t.Fatal("credential stored as plaintext")
	}
}
