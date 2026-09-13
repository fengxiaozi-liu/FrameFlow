package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultEncryptsCredentials(t *testing.T) {
	dir := t.TempDir()
	v, e := OpenVault(dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = v.Put("provider-a", "sk-secret"); e != nil {
		t.Fatal(e)
	}
	value, ok := v.Get("provider-a")
	if !ok || value != "sk-secret" {
		t.Fatal(value)
	}
	raw, e := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if e != nil || strings.Contains(string(raw), "sk-secret") {
		t.Fatal("credential stored as plaintext")
	}
}
