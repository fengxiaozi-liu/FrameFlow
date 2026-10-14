package mediaruntime

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func testArchive(t *testing.T, content string, corruptHash bool) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	hashes := map[string]string{}
	for _, name := range toolFiles {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		value := []byte(name + ":" + content)
		if _, err := file.Write(value); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(value)
		hashes[name] = hex.EncodeToString(hash[:])
	}
	if corruptHash {
		hashes["ffmpeg.exe"] = hex.EncodeToString(make([]byte, sha256.Size))
	}
	manifest, err := writer.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(manifest).Encode(struct {
		Files map[string]string `json:"files"`
	}{hashes}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestPrepareExtractsReusesAndRepairsTools(t *testing.T) {
	root := filepath.Join(t.TempDir(), "中文 runtime")
	archive := testArchive(t, "release-one", false)
	paths, err := prepareArchive(root, archive)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(paths.FFmpeg) || !filepath.IsAbs(paths.FFprobe) {
		t.Fatal("media tools must have absolute paths", paths)
	}
	before, err := os.Stat(paths.FFprobe)
	if err != nil {
		t.Fatal(err)
	}
	reused, err := prepareArchive(root, archive)
	if err != nil || reused != paths {
		t.Fatal(reused, err)
	}
	after, err := os.Stat(paths.FFprobe)
	if err != nil || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("valid cached tool was rewritten", err)
	}
	if err := os.WriteFile(paths.FFprobe, []byte("corrupt cache"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(paths.FFmpeg); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareArchive(root, archive); err != nil {
		t.Fatal(err)
	}
	for _, name := range toolFiles {
		value, err := os.ReadFile(filepath.Join(filepath.Dir(paths.FFmpeg), name))
		if err != nil || string(value) != name+":release-one" {
			t.Fatalf("%s not restored: %q %v", name, value, err)
		}
	}
}

func TestPrepareKeepsVersionsSeparate(t *testing.T) {
	root := t.TempDir()
	first, err := prepareArchive(root, testArchive(t, "one", false))
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepareArchive(root, testArchive(t, "two", false))
	if err != nil || first.FFmpeg == second.FFmpeg {
		t.Fatal("versions share a mutable executable", second, err)
	}
	value, err := os.ReadFile(first.FFmpeg)
	if err != nil || string(value) != "ffmpeg.exe:one" {
		t.Fatal("upgrade changed the old tool", err)
	}
}

func TestPrepareRejectsCorruptPayloadAndUnwritableDirectory(t *testing.T) {
	root := t.TempDir()
	if _, err := prepareArchive(root, []byte("invalid zip")); err == nil {
		t.Fatal("invalid archive accepted")
	}
	if _, err := prepareArchive(root, testArchive(t, "one", true)); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "blocked"), []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareArchive(filepath.Join(root, "blocked"), testArchive(t, "one", false)); err == nil {
		t.Fatal("invalid cache directory accepted")
	}
	entries, err := filepath.Glob(filepath.Join(root, "*", ".media-tool-*"))
	if err != nil || len(entries) != 0 {
		t.Fatal("failed extraction left partial files", entries, err)
	}
}

func TestConcurrentPreparationProducesCompleteTools(t *testing.T) {
	root := t.TempDir()
	archive := testArchive(t, "concurrent", false)
	var group sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < cap(errors); i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := prepareArchive(root, archive)
			errors <- err
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	paths, err := prepareArchive(root, archive)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(paths.FFmpeg))
	if err != nil || len(entries) != len(toolFiles) {
		t.Fatal("unexpected partial files", entries, err)
	}
}
