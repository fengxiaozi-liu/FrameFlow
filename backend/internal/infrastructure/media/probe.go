package media

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func executable(envName, fallback string) string {
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return fallback
	}
	return value
}

func Probe(ctx context.Context, path string) (task.MediaProbeResult, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(probeCtx, executable("FRAMEFLOW_FFPROBE_PATH", "ffprobe"),
		"-v", "error", "-show_entries", "format=duration:stream=codec_type,width,height",
		"-of", "json", path).Output()
	if err != nil {
		return task.MediaProbeResult{}, errors.New("ffprobe failed to read media")
	}
	var payload struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return task.MediaProbeResult{}, err
	}
	duration, err := strconv.ParseFloat(payload.Format.Duration, 64)
	if err != nil || duration <= 0 {
		return task.MediaProbeResult{}, errors.New("media duration is unavailable")
	}
	result := task.MediaProbeResult{Duration: duration}
	for _, stream := range payload.Streams {
		if stream.CodecType == "video" {
			result.Width, result.Height = stream.Width, stream.Height
		}
		if stream.CodecType == "audio" {
			result.HasAudio = true
		}
	}
	return result, nil
}
