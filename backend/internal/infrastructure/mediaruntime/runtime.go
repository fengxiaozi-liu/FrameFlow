// Package mediaruntime prepares the media tools included in the Windows executable.
package mediaruntime

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Paths struct {
	FFmpeg  string
	FFprobe string
}

var toolFiles = []string{"ffmpeg.exe", "ffprobe.exe", "FFmpeg-LICENSE.txt", "FFmpeg-NOTICE.txt"}

// Prepare returns empty paths for development and non-embedded builds. Release
// tools use a content-addressed directory so upgrades cannot replace tools used
// by another running version. Windows needs executable files on disk to run them.
func Prepare(directory string) (Paths, error) {
	archive := embeddedArchive()
	if len(archive) == 0 {
		return Paths{}, nil
	}
	return prepareArchive(directory, archive)
}

func prepareArchive(directory string, archive []byte) (Paths, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return Paths{}, fmt.Errorf("read embedded media tools: %w", err)
	}
	files := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		if _, exists := files[file.Name]; exists {
			return Paths{}, errors.New("duplicate embedded media file")
		}
		files[file.Name] = file
	}
	if len(files) != len(toolFiles)+1 || files["manifest.json"] == nil {
		return Paths{}, errors.New("incomplete embedded media tools")
	}
	manifestFile, err := files["manifest.json"].Open()
	if err != nil {
		return Paths{}, err
	}
	var manifest struct {
		Files map[string]string `json:"files"`
	}
	err = json.NewDecoder(io.LimitReader(manifestFile, 64<<10)).Decode(&manifest)
	_ = manifestFile.Close()
	if err != nil || len(manifest.Files) != len(toolFiles) {
		return Paths{}, errors.New("invalid embedded media manifest")
	}
	for _, name := range toolFiles {
		digest, err := hex.DecodeString(manifest.Files[name])
		if files[name] == nil || err != nil || len(digest) != sha256.Size || files[name].UncompressedSize64 > 512<<20 {
			return Paths{}, fmt.Errorf("invalid embedded media file: %s", name)
		}
	}
	archiveHash := sha256.Sum256(archive)
	cacheDir, err := filepath.Abs(filepath.Join(directory, hex.EncodeToString(archiveHash[:])))
	if err != nil {
		return Paths{}, err
	}
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return Paths{}, fmt.Errorf("create media tool directory: %w", err)
	}
	for _, name := range toolFiles {
		path := filepath.Join(cacheDir, name)
		if err := ensureFile(path, files[name], manifest.Files[name]); err != nil {
			return Paths{}, fmt.Errorf("prepare %s: %w", name, err)
		}
	}
	return Paths{FFmpeg: filepath.Join(cacheDir, "ffmpeg.exe"), FFprobe: filepath.Join(cacheDir, "ffprobe.exe")}, nil
}

func matchesHash(path, expected string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	return err == nil && hex.EncodeToString(hash.Sum(nil)) == expected
}

func ensureFile(path string, source *zip.File, expected string) error {
	if matchesHash(path, expected) {
		return nil
	}
	input, err := source.Open()
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.CreateTemp(filepath.Dir(path), ".media-tool-*")
	if err != nil {
		return err
	}
	temporary := output.Name()
	defer os.Remove(temporary)
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(output, hash), io.LimitReader(input, (512<<20)+1))
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return errors.New("embedded media checksum mismatch")
	}
	if err := os.Chmod(temporary, 0750); err != nil {
		return err
	}
	// Concurrent startup can briefly lock the destination on Windows. Recheck
	// before replacing it, and wait a bounded time for another writer to finish.
	for attempt := 0; attempt < 20; attempt++ {
		if matchesHash(path, expected) {
			return nil
		}
		if err = os.Rename(temporary, path); err == nil {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return err
}
