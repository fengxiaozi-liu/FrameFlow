package media

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type VoiceSegment struct {
	Path         string
	PauseAfterMS int
}

func MixVoiceTrack(ctx context.Context, segments []VoiceSegment, maxDuration float64, destination string) (float64, error) {
	if len(segments) == 0 {
		return 0, errors.New("voice track requires segments")
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	filters := []string{}
	labels := ""
	duration := 0.0
	for i, s := range segments {
		if s.PauseAfterMS < 0 || s.PauseAfterMS > 10000 {
			return 0, errors.New("invalid pause")
		}
		p, err := Probe(ctx, s.Path)
		if err != nil || !p.HasAudio {
			return 0, errors.New("invalid voice segment")
		}
		args = append(args, "-i", s.Path)
		pause := float64(s.PauseAfterMS) / 1000
		duration += p.Duration + pause
		filters = append(filters, fmt.Sprintf("[%d:a]aresample=24000,aformat=sample_fmts=fltp:channel_layouts=mono,apad=pad_len=%d[a%d]", i, s.PauseAfterMS*24, i))
		labels += fmt.Sprintf("[a%d]", i)
	}
	if duration > maxDuration+0.05 {
		return duration, fmt.Errorf("audio_exceeds_duration: %.2fs > %.2fs", duration, maxDuration)
	}
	filters = append(filters, fmt.Sprintf("%sconcat=n=%d:v=0:a=1[out]", labels, len(segments)))
	args = append(args, "-filter_complex", strings.Join(filters, ";"), "-map", "[out]", "-c:a", "pcm_s16le", destination)
	if out, err := exec.CommandContext(ctx, executable("FRAMEFLOW_FFMPEG_PATH", "ffmpeg"), args...).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("voice mix failed: %s", out)
	}
	return duration, nil
}
