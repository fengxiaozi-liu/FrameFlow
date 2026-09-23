package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMediaRuntimeFailureDoesNotBlockDraftStorage(t *testing.T) {
	base := t.TempDir()
	t.Setenv("FRAMEFLOW_FFMPEG_PATH", filepath.Join(base, "missing-ffmpeg"))
	t.Setenv("FRAMEFLOW_FFPROBE_PATH", filepath.Join(base, "missing-ffprobe"))
	t.Setenv("FRAMEFLOW_AUDIO_OBJECT_BASE_URL", "")
	status := checkMediaRuntime(base)
	if status.CompositionAvailable || status.AudioDeliveryAvailable {
		t.Fatalf("unconfigured media features reported ready: %+v", status)
	}
	if info, err := os.Stat(status.WorkDir); err != nil || !info.IsDir() {
		t.Fatalf("media work directory unavailable: %v", err)
	}
}

func TestAudioDeliveryRequiresCompleteHTTPSConfiguration(t *testing.T) {
	base := t.TempDir()
	t.Setenv("FRAMEFLOW_FFMPEG_PATH", filepath.Join(base, "missing-ffmpeg"))
	t.Setenv("FRAMEFLOW_FFPROBE_PATH", filepath.Join(base, "missing-ffprobe"))
	t.Setenv("FRAMEFLOW_AUDIO_OBJECT_BASE_URL", "http://example.com")
	t.Setenv("FRAMEFLOW_AUDIO_OBJECT_BUCKET", "audio")
	t.Setenv("FRAMEFLOW_AUDIO_OBJECT_ACCESS_KEY_ID", "example")
	t.Setenv("FRAMEFLOW_AUDIO_OBJECT_ACCESS_KEY_SECRET", "example")
	if checkMediaRuntime(base).AudioDeliveryAvailable {
		t.Fatal("insecure object storage URL accepted")
	}
	t.Setenv("FRAMEFLOW_AUDIO_OBJECT_BASE_URL", "https://example.com")
	if !checkMediaRuntime(base).AudioDeliveryAvailable {
		t.Fatal("complete object storage configuration was not detected")
	}
}
