package application

import (
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
)

func inspectUploadedMedia(ctx context.Context, path, contentType string) (material.MediaInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return material.MediaInfo{}, err
	}
	info := material.MediaInfo{Format: contentType, SizeBytes: stat.Size()}
	ext := strings.ToLower(filepath.Ext(path))
	if strings.HasPrefix(contentType, "image/") {
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" {
			return info, errors.New("supported image extensions: PNG, JPEG, GIF")
		}
		file, err := os.Open(path)
		if err != nil {
			return info, err
		}
		defer file.Close()
		config, _, err := image.DecodeConfig(file)
		if err != nil {
			return info, err
		}
		info.Width, info.Height = config.Width, config.Height
		return info, nil
	}
	if ext != ".wav" && ext != ".mp3" {
		return info, errors.New("supported audio extensions: WAV, MP3")
	}
	probe := os.Getenv("FRAMEFLOW_FFPROBE_PATH")
	if probe == "" {
		probe = "ffprobe"
	}
	output, err := exec.CommandContext(ctx, probe, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return info, errors.New("ffprobe is required to read audio duration")
	}
	info.DurationSeconds, err = strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil || info.DurationSeconds <= 0 {
		return info, errors.New("audio duration is unavailable")
	}
	return info, nil
}
