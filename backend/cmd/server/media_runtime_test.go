package main

import (
	"context"
	"errors"
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

func TestMediaVerificationRecoversTransientFailureAndBoundsPermanentFailure(t *testing.T) {
	attempts := 0
	transient := errors.New("local startup failed")
	if err := verifyMediaTool(context.Background(), func(context.Context) error {
		attempts++
		if attempts == 1 {
			return transient
		}
		return nil
	}); err != nil || attempts != 2 {
		t.Fatal(attempts, err)
	}
	attempts = 0
	if err := verifyMediaTool(context.Background(), func(context.Context) error { attempts++; return transient }); !errors.Is(err, transient) || attempts != 3 {
		t.Fatal(attempts, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts = 0
	if err := verifyMediaTool(ctx, func(context.Context) error { attempts++; return nil }); !errors.Is(err, context.Canceled) || attempts != 0 {
		t.Fatal(attempts, err)
	}
}
