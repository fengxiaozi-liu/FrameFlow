package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
)

func (s TaskService) CreateComposition(g *gin.Context) {
	var input struct {
		ExpectedVersion *int64  `json:"expected_version"`
		IdempotencyKey  string  `json:"idempotency_key"`
		MusicMaterialID string  `json:"music_material_id"`
		MusicVolume     float64 `json:"music_volume"`
		SourceVolume    float64 `json:"source_volume"`
		VoiceVolume     float64 `json:"voice_volume"`
		AspectRatio     string  `json:"aspect_ratio"`
		Resolution      string  `json:"resolution"`
	}
	if err := g.ShouldBindJSON(&input); err != nil {
		writeError(g, Invalid("invalid_composition", err))
		return
	}
	if input.ExpectedVersion == nil || *input.ExpectedVersion < 0 || input.IdempotencyKey == "" {
		writeError(g, Invalid("invalid_composition", errors.New("expected_version and idempotency_key are required")))
		return
	}
	if input.AspectRatio != "16:9" && input.AspectRatio != "9:16" || input.Resolution != "720P" && input.Resolution != "1080P" ||
		input.MusicVolume < 0 || input.MusicVolume > 1 || input.SourceVolume < 0 || input.SourceVolume > 1 || input.VoiceVolume < 0 || input.VoiceVolume > 1 {
		writeError(g, Invalid("invalid_composition", errors.New("unsupported output settings or volume")))
		return
	}
	projectID, draftID := g.Param("id"), g.Param("draftId")
	identity := sha256.Sum256([]byte(projectID + "\x00" + draftID + "\x00" + input.IdempotencyKey))
	taskID := "compose-" + hex.EncodeToString(identity[:16])
	if existing, err := s.Store.Get(g.Request.Context(), taskID); err == nil {
		if existing.InputVersion != *input.ExpectedVersion {
			writeError(g, Conflict("idempotency_key_reused", errors.New("idempotency key used for another revision")))
			return
		}
		g.JSON(http.StatusAccepted, existing)
		return
	}
	if s.Probe == nil {
		writeError(g, unavailable("ffprobe_unavailable", "composition requires ffprobe"))
		return
	}
	p, err := s.Projects.Get(g.Request.Context(), projectID)
	if err != nil {
		writeError(g, err)
		return
	}
	d, err := draftByID(&p, draftID)
	if err != nil {
		writeError(g, Invalid("draft_not_found", err))
		return
	}
	if d.Version != *input.ExpectedVersion {
		writeError(g, Conflict("draft_version_conflict", project.ErrVersionConflict))
		return
	}
	snapshot := task.CompositionInput{
		MusicMaterialID: input.MusicMaterialID, MusicVolume: input.MusicVolume,
		SourceVolume: input.SourceVolume, VoiceVolume: input.VoiceVolume,
		AspectRatio: input.AspectRatio, Resolution: input.Resolution,
	}
	issues := []VideoValidationIssue{}
	add := func(sceneID, field, message string) {
		issues = append(issues, VideoValidationIssue{SceneID: sceneID, Field: field, Message: message})
	}
	if len(d.Story.Scenes) == 0 {
		add("", "scenes", "成片需要至少一个镜头")
	}
	for _, scene := range d.Story.Scenes {
		selectedID := d.SelectedVersions[scene.ID]
		if selectedID == "" {
			add(scene.ID, "selected_version", "请选择已保存的视频版本")
			continue
		}
		var chosen *project.VideoVersion
		for i := range d.VideoVersions {
			if d.VideoVersions[i].ID == selectedID && d.VideoVersions[i].SceneID == scene.ID {
				chosen = &d.VideoVersions[i]
				break
			}
		}
		if chosen == nil {
			add(scene.ID, "selected_version", "选用版本不存在")
			continue
		}
		path, err := localMediaPath(s.UploadDir, chosen.LocalMediaPath)
		if err != nil {
			add(scene.ID, "clip", "视频文件路径无效")
			continue
		}
		probe, err := s.Probe(g.Request.Context(), path)
		if err != nil || probe.Width <= 0 || probe.Height <= 0 {
			add(scene.ID, "clip", "视频文件不可用或无法读取真实时长")
			continue
		}
		clip := task.CompositionClip{SceneID: scene.ID, VersionID: selectedID, MediaPath: chosen.LocalMediaPath, DurationSeconds: probe.Duration}
		for _, binding := range d.Bindings {
			if binding.SceneID != scene.ID || binding.Usage != "voiceover" {
				continue
			}
			asset, err := s.Materials.Get(g.Request.Context(), binding.MaterialID)
			if err != nil || asset.Kind != material.Voice {
				add(scene.ID, "voiceover", "后期配音素材不可用")
				continue
			}
			voicePath, err := localMediaPath(s.UploadDir, asset.URL)
			if err != nil {
				add(scene.ID, "voiceover", "后期配音文件不可用")
				continue
			}
			voice, err := s.Probe(g.Request.Context(), voicePath)
			if err != nil || !voice.HasAudio || voice.Duration > probe.Duration+0.05 {
				add(scene.ID, "voiceover", "配音超过镜头真实时长或文件不可用")
				continue
			}
			clip.VoicePath = asset.URL
		}
		snapshot.Clips = append(snapshot.Clips, clip)
	}
	if input.MusicMaterialID != "" {
		asset, err := s.Materials.Get(g.Request.Context(), input.MusicMaterialID)
		if err != nil || asset.Kind != material.Music {
			add("", "music_material_id", "背景音乐不可用")
		} else {
			musicPath, pathErr := localMediaPath(s.UploadDir, asset.URL)
			if pathErr != nil {
				add("", "music_material_id", "背景音乐文件不存在")
			} else if probe, probeErr := s.Probe(g.Request.Context(), musicPath); probeErr != nil || !probe.HasAudio {
				add("", "music_material_id", "背景音乐无法读取")
			} else {
				snapshot.MusicPath = asset.URL
			}
		}
	}
	if len(issues) > 0 {
		g.JSON(http.StatusUnprocessableEntity, gin.H{"code": "composition_input_invalid", "issues": issues})
		return
	}
	encoded, _ := json.Marshal(snapshot)
	digest := sha256.Sum256(encoded)
	now := time.Now().UTC()
	created := task.New(taskID, task.KindComposition, task.Input{Prompt: "compose"}, now)
	created.ProjectID, created.DraftID = projectID, draftID
	created.InputVersion, created.IdempotencyKey = *input.ExpectedVersion, input.IdempotencyKey
	created.InputHash = hex.EncodeToString(digest[:])
	created.Composition = &snapshot
	if err := s.Store.Save(g.Request.Context(), created); err != nil {
		writeError(g, err)
		return
	}
	s.broadcast(g.Request.Context(), created)
	if s.Enqueuer != nil {
		if err := s.Enqueuer.Enqueue(g.Request.Context(), created); err != nil {
			writeError(g, s.rejectSubmission(g.Request.Context(), created, err))
			return
		}
	}
	g.JSON(http.StatusAccepted, created)
}

func localMediaPath(dir, ref string) (string, error) {
	if dir == "" || !strings.HasPrefix(ref, "/media/") {
		return "", errors.New("local media reference required")
	}
	name := strings.TrimPrefix(ref, "/media/")
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, "/\\") {
		return "", errors.New("invalid media name")
	}
	path := filepath.Join(dir, name)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("media file is missing: %s", name)
	}
	return path, nil
}
