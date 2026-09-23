package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type Composer struct {
	UploadDir string
	WorkDir   string
}

func (c Composer) Compose(ctx context.Context, id string, input task.CompositionInput, progress func(int, task.Stage)) (string, float64, error) {
	if len(input.Clips) == 0 {
		return "", 0, errors.New("composition has no clips")
	}
	if c.UploadDir == "" || c.WorkDir == "" {
		return "", 0, errors.New("media directories are not configured")
	}
	if err := os.MkdirAll(c.WorkDir, 0750); err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(c.UploadDir, 0750); err != nil {
		return "", 0, err
	}
	destination := filepath.Join(c.UploadDir, "composition-"+id+".mp4")
	if _, err := os.Stat(destination); err == nil {
		probe, err := Probe(ctx, destination)
		if err != nil || probe.Width <= 0 {
			return "", 0, errors.New("saved composition file is invalid")
		}
		return "/media/" + filepath.Base(destination), probe.Duration, nil
	}
	work, err := os.MkdirTemp(c.WorkDir, "composition-*")
	if err != nil {
		return "", 0, err
	}
	defer os.RemoveAll(work)
	width, height, err := outputSize(input.AspectRatio, input.Resolution)
	if err != nil {
		return "", 0, err
	}
	clips := make([]string, 0, len(input.Clips))
	total := float64(0)
	for index, clip := range input.Clips {
		source, err := resolveLocal(c.UploadDir, clip.MediaPath)
		if err != nil {
			return "", 0, fmt.Errorf("scene %s clip: %w", clip.SceneID, err)
		}
		probed, err := Probe(ctx, source)
		if err != nil || probed.Width <= 0 || probed.Height <= 0 {
			return "", 0, fmt.Errorf("scene %s clip cannot be probed", clip.SceneID)
		}
		if clip.DurationSeconds <= 0 || abs(probed.Duration-clip.DurationSeconds) > 0.2 {
			return "", 0, fmt.Errorf("scene %s clip duration changed after submission", clip.SceneID)
		}
		target := filepath.Join(work, fmt.Sprintf("clip-%04d.mp4", index))
		if err := c.normalizeClip(ctx, source, clip, probed, width, height, input.SourceVolume, input.VoiceVolume, target); err != nil {
			return "", 0, fmt.Errorf("scene %s normalize: %w", clip.SceneID, err)
		}
		clips = append(clips, target)
		total += probed.Duration
		progress(10+int(55*float64(index+1)/float64(len(input.Clips))), task.StageComposing)
	}
	joined := filepath.Join(work, "joined.mp4")
	if err := concatClips(ctx, clips, joined); err != nil {
		return "", 0, err
	}
	progress(75, task.StageComposing)
	output := joined
	if input.MusicPath != "" {
		music, err := resolveLocal(c.UploadDir, input.MusicPath)
		if err != nil {
			return "", 0, fmt.Errorf("background music: %w", err)
		}
		output = filepath.Join(work, "mixed.mp4")
		filter := fmt.Sprintf("[0:a]anull[a0];[1:a]volume=%.3f[a1];[a0][a1]amix=inputs=2:duration=first:dropout_transition=0[a]", input.MusicVolume)
		args := []string{"-y", "-i", joined, "-stream_loop", "-1", "-i", music, "-filter_complex", filter, "-map", "0:v:0", "-map", "[a]", "-t", fmt.Sprintf("%.3f", total), "-c:v", "copy", "-c:a", "aac", "-ar", "48000", "-ac", "2", output}
		if err := runFFmpeg(ctx, args...); err != nil {
			return "", 0, fmt.Errorf("music mix: %w", err)
		}
	}
	progress(90, task.StageComposing)
	staged, err := os.CreateTemp(c.UploadDir, ".composition-*.mp4")
	if err != nil {
		return "", 0, err
	}
	defer os.Remove(staged.Name())
	source, err := os.Open(output)
	if err != nil {
		_ = staged.Close()
		return "", 0, err
	}
	_, copyErr := io.Copy(staged, source)
	closeSourceErr := source.Close()
	closeStagedErr := staged.Close()
	if copyErr != nil {
		return "", 0, copyErr
	}
	if closeSourceErr != nil {
		return "", 0, closeSourceErr
	}
	if closeStagedErr != nil {
		return "", 0, closeStagedErr
	}
	finalProbe, err := Probe(ctx, staged.Name())
	if err != nil || finalProbe.Width != width || finalProbe.Height != height || abs(finalProbe.Duration-total) > 0.5 {
		return "", 0, errors.New("composed output failed duration or resolution check")
	}
	if err := os.Rename(staged.Name(), destination); err != nil {
		return "", 0, err
	}
	return "/media/" + filepath.Base(destination), finalProbe.Duration, nil
}

func (c Composer) normalizeClip(ctx context.Context, source string, clip task.CompositionClip, probed task.MediaProbeResult, width, height int, sourceVolume, voiceVolume float64, target string) error {
	args := []string{"-y", "-i", source}
	voiceIndex := -1
	if clip.VoicePath != "" {
		voice, err := resolveLocal(c.UploadDir, clip.VoicePath)
		if err != nil {
			return err
		}
		voiceProbe, err := Probe(ctx, voice)
		if err != nil || !voiceProbe.HasAudio || voiceProbe.Duration > probed.Duration+0.05 {
			return errors.New("voiceover is longer than its clip")
		}
		voiceIndex = 1
		args = append(args, "-i", voice)
	}
	sourceAudioIndex := 0
	if !probed.HasAudio {
		if voiceIndex >= 0 {
			sourceAudioIndex = 2
		} else {
			sourceAudioIndex = 1
		}
		args = append(args, "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000")
	}
	videoFilter := fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black,fps=30,format=yuv420p[v]", width, height, width, height)
	audioFilter := fmt.Sprintf("[%d:a]volume=%.3f[base]", sourceAudioIndex, sourceVolume)
	if voiceIndex >= 0 {
		audioFilter += fmt.Sprintf(";[%d:a]volume=%.3f[voice];[base][voice]amix=inputs=2:duration=first:dropout_transition=0[a]", voiceIndex, voiceVolume)
	} else {
		audioFilter += ";[base]anull[a]"
	}
	args = append(args, "-filter_complex", videoFilter+";"+audioFilter, "-map", "[v]", "-map", "[a]", "-t", fmt.Sprintf("%.3f", probed.Duration), "-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p", "-c:a", "aac", "-ar", "48000", "-ac", "2", "-movflags", "+faststart", target)
	return runFFmpeg(ctx, args...)
}

func concatClips(ctx context.Context, clips []string, target string) error {
	args := []string{"-y"}
	for _, clip := range clips {
		args = append(args, "-i", clip)
	}
	labels := strings.Builder{}
	for index := range clips {
		labels.WriteString(fmt.Sprintf("[%d:v][%d:a]", index, index))
	}
	filter := fmt.Sprintf("%sconcat=n=%d:v=1:a=1[v][a]", labels.String(), len(clips))
	args = append(args, "-filter_complex", filter, "-map", "[v]", "-map", "[a]", "-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p", "-c:a", "aac", "-movflags", "+faststart", target)
	return runFFmpeg(ctx, args...)
}

func runFFmpeg(ctx context.Context, args ...string) error {
	command := exec.CommandContext(ctx, executable("FRAMEFLOW_FFMPEG_PATH", "ffmpeg"), args...)
	output, err := command.CombinedOutput()
	if err != nil {
		tail := string(output)
		if len(tail) > 800 {
			tail = tail[len(tail)-800:]
		}
		return fmt.Errorf("ffmpeg: %w: %s", err, tail)
	}
	return nil
}

func resolveLocal(root, ref string) (string, error) {
	if !strings.HasPrefix(ref, "/media/") {
		return "", errors.New("media must be stored locally")
	}
	name := strings.TrimPrefix(ref, "/media/")
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, "/\\") {
		return "", errors.New("invalid media path")
	}
	value := filepath.Join(root, name)
	info, err := os.Stat(value)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("media file is missing")
	}
	return value, nil
}
func outputSize(aspect, resolution string) (int, int, error) {
	size := 720
	if resolution == "1080P" {
		size = 1080
	} else if resolution != "720P" {
		return 0, 0, errors.New("unsupported resolution")
	}
	switch aspect {
	case "16:9":
		return size * 16 / 9, size, nil
	case "9:16":
		return size, size * 16 / 9, nil
	default:
		return 0, 0, errors.New("unsupported aspect ratio")
	}
}
func abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
