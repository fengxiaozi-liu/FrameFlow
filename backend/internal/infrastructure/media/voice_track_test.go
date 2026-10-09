package media

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestVoiceTrackTimingAndOverflow(t *testing.T) {
	if os.Getenv("FRAMEFLOW_FFMPEG_PATH") == "" {
		t.Skip("requires configured ffmpeg/ffprobe integration runtime")
	}
	source := filepath.Join(t.TempDir(), "voice.wav")
	if err := runFFmpeg(context.Background(), "-y", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "2", source); err != nil {
		t.Fatal(err)
	}
	probe, err := Probe(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "mixed.wav")
	segments := []VoiceSegment{{Path: source, PauseAfterMS: 200}, {Path: source}}
	duration, err := MixVoiceTrack(context.Background(), segments, probe.Duration*2+1, out)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Probe(context.Background(), out)
	if err != nil || !result.HasAudio || abs(result.Duration-duration) > .05 {
		t.Fatal(result, duration, err)
	}
	if _, err = MixVoiceTrack(context.Background(), segments, 1, filepath.Join(t.TempDir(), "overflow.wav")); err == nil {
		t.Fatal("overflow was silently cropped")
	}
}
