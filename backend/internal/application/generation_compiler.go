package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
)

type GenerationRequest struct {
	VideoInputMode       string `json:"video_input_mode,omitempty"`
	FirstFrameMaterialID string `json:"first_frame_material_id,omitempty"`
	LastFrameMaterialID  string `json:"last_frame_material_id,omitempty"`
	ExpectedVersion      *int64 `json:"expected_version"`
	IdempotencyKey       string `json:"idempotency_key"`
	ModelID              string `json:"model_id"`
	VideoModelID         string `json:"video_model_id,omitempty"`
	Path                 string `json:"path"`
	AspectRatio          string `json:"aspect_ratio"`
	Resolution           string `json:"resolution"`
	DurationSeconds      int    `json:"duration_seconds"`
	UtteranceID          string `json:"utterance_id"`
}

type GenerationCompiler struct {
	Models            provider.ModelRepository
	Materials         material.Repository
	Directory         string
	MaxReferenceBytes int64
}

func (c GenerationCompiler) Compile(ctx context.Context, p project.Project, d project.Draft, scene story.Scene, in GenerationRequest) (task.GenerationSnapshot, error) {
	s := task.GenerationSnapshot{SchemaVersion: 1, ProjectID: p.ID, DraftID: d.ID, SceneID: scene.ID, DraftVersion: d.Version, ModelID: in.ModelID, Path: in.Path, AspectRatio: in.AspectRatio, Resolution: in.Resolution, DurationSeconds: in.DurationSeconds, CreatedAt: time.Now().UTC(), Entities: []task.SnapshotEntity{}, Utterances: []story.Utterance{}}
	if in.ExpectedVersion == nil || *in.ExpectedVersion != d.Version {
		return s, project.ErrVersionConflict
	}
	if c.Models == nil {
		return s, errors.New("model catalog unavailable")
	}
	m, err := c.Models.GetModel(ctx, in.ModelID)
	if err != nil {
		return s, err
	}
	cap, ok := m.InputCapability()
	if !ok || !m.Enabled || !m.Supported {
		return s, errors.New("model input capability unavailable")
	}
	c.MaxReferenceBytes = provider.MaxLocalImageBytes
	if in.Path == "reference_video" || in.Path == "first_frame_video" {
		c.MaxReferenceBytes = m.Operations[provider.Video].MaxImageBytes
	}
	s.ModelConfig = &m
	s.CapabilityVersion = cap.Version
	s.DependencyHash = dependencyHash(p.EntityCatalog, scene, in.Path, in.UtteranceID)
	resolved, err := ResolveBindings(p.EntityCatalog, scene, in.Path == "speech")
	if err != nil {
		return s, err
	}
	s.Prompt = resolved.Prompt
	if in.Path == "speech" {
		if !cap.Speech || !m.Supports(provider.Speech) {
			return s, errors.New("model does not support speech")
		}
		for _, u := range scene.Utterances {
			if u.ID == in.UtteranceID {
				v := resolved.Voices[u.ID]
				if v.ModelID != m.ID {
					return s, errors.New("speech model must match selected voice profile")
				}
				if u.Emotion != "" && v.Instructions == "" {
					return s, errors.New("this speech model does not support emotion instructions")
				}
				s.Utterances = []story.Utterance{u}
				s.Prompt = u.Text
				s.Speech = []task.SpeechInput{{VoiceProfileID: v.ID, VoiceRevision: v.Revision, ModelID: v.ModelID, Voice: v.RemoteVoiceID, Language: v.Language, Text: u.Text, Instructions: v.Instructions}}
			}
		}
		if len(s.Speech) != 1 {
			return s, errors.New("utterance not found")
		}
		return task.FreezeSnapshot(s)
	}
	if strings.TrimSpace(s.Prompt) == "" {
		return s, errors.New("shot description is required")
	}
	if s.AspectRatio == "" {
		s.AspectRatio = "16:9"
	}
	if s.AspectRatio != "16:9" && s.AspectRatio != "9:16" && s.AspectRatio != "1:1" {
		return s, errors.New("unsupported aspect ratio")
	}
	if in.Path == "reference_image_then_video" || in.Path == "reference_video" {
		directVideo := in.Path == "reference_video"
		var videoCap provider.VideoCapability
		if directVideo {
			videoCap, err = m.VideoCapabilityFor(in.VideoInputMode)
			if err != nil {
				return s, err
			}
			s.VideoInputMode = videoCap.InputMode
			if s.VideoInputMode == "reference_images" && (in.FirstFrameMaterialID != "" || in.LastFrameMaterialID != "") {
				return s, errors.New("智能参考不能同时提交首尾帧")
			}
			if s.VideoInputMode == "first_frame" {
				s.FirstFrameMaterialID, s.LastFrameMaterialID = in.FirstFrameMaterialID, in.LastFrameMaterialID
				if in.VideoInputMode != "" && s.FirstFrameMaterialID == "" {
					return s, errors.New("请选择首帧图片")
				}
				if s.LastFrameMaterialID != "" && videoCap.AllowedMedia["last_frame"] == 0 {
					return s, errors.New("模型不支持尾帧图片")
				}
			}
		}
		if directVideo && (!m.Supports(provider.Video) || (!cap.ReferenceVideo && !cap.FirstFrame)) {
			return s, errors.New("请选择支持参考图直接生成视频的模型")
		}
		if !directVideo && (!m.Supports(provider.Image) || m.Operations[provider.Image].MaxReferences == 0) {
			return s, errors.New("choose a verified multi-image model")
		}
		operation := provider.Image
		if directVideo {
			operation = provider.Video
		}
		if !slices.Contains(m.Operations[operation].Ratios, s.AspectRatio) {
			return s, errors.New("unsupported aspect ratio for configured model")
		}
		count := 0
		selectedReferences := map[string]bool{}
		for _, b := range resolved.Entities {
			e := task.SnapshotEntity{BindingID: b.Binding.ID, EntityID: b.Entity.ID, Revision: b.Entity.Revision, Description: b.Description, References: []task.MediaReference{}}
			s.Prompt += fmt.Sprintf("\n%s（%s，%s）：%s", b.Entity.Name, b.Entity.Kind, b.Binding.Role, b.Description)
			for _, r := range b.References {
				if directVideo && s.VideoInputMode == "first_frame" && s.FirstFrameMaterialID != "" && r.MaterialID != s.FirstFrameMaterialID && r.MaterialID != s.LastFrameMaterialID {
					continue
				}
				if directVideo && selectedReferences[r.MaterialID] {
					continue
				}
				selectedReferences[r.MaterialID] = true
				ref, err := c.reference(ctx, r.MaterialID, r.Label)
				if err != nil {
					return s, err
				}
				e.References = append(e.References, ref)
				count++
				if directVideo && s.VideoInputMode == "first_frame" && s.FirstFrameMaterialID != "" {
					role := "首帧"
					if r.MaterialID == s.LastFrameMaterialID {
						role = "尾帧"
						if r.MaterialID == s.FirstFrameMaterialID {
							role = "首帧和尾帧"
						}
					}
					s.Prompt += fmt.Sprintf("\n%s属于%s；视角：%s。", role, b.Entity.Name, r.Label)
				} else {
					s.Prompt += fmt.Sprintf("\n图%d属于%s；视角：%s。", count, b.Entity.Name, r.Label)
				}
			}
			s.Entities = append(s.Entities, e)
		}
		if (!directVideo || s.VideoInputMode == "reference_images") && count > m.Operations[operation].MaxReferences {
			return s, fmt.Errorf("reference_image_limit: 已选 %d 张，模型最多支持 %d 张；请移除本镜头多余参考图后重试", count, m.Operations[operation].MaxReferences)
		}
		if directVideo {
			if count == 0 {
				return s, errors.New("reference_image_required: 请在引用素材中上传至少一张参考图片")
			}
			if s.DurationSeconds == 0 {
				s.DurationSeconds = scene.DurationSeconds
			}
			if s.Resolution == "" {
				s.Resolution = "720P"
			}
			usages := make([]string, count)
			for i := range usages {
				usages[i] = "reference_image"
			}
			if s.VideoInputMode == "first_frame" {
				if s.FirstFrameMaterialID == "" && in.VideoInputMode == "" && count == 1 {
					for id := range selectedReferences {
						s.FirstFrameMaterialID = id
					}
				}
				if !selectedReferences[s.FirstFrameMaterialID] {
					return s, errors.New("首帧必须选择当前分镜引用中的图片")
				}
				usages = []string{"first_frame"}
				if s.LastFrameMaterialID != "" {
					if !selectedReferences[s.LastFrameMaterialID] {
						return s, errors.New("尾帧必须选择当前分镜引用中的图片")
					}
					usages = append(usages, "last_frame")
				}
				if in.VideoInputMode == "" && count > len(usages) {
					return s, errors.New("请明确选择首帧与尾帧图片")
				}
			}
			if err := videoCap.ValidateInput(s.DurationSeconds, s.Resolution, usages); err != nil {
				return s, err
			}
			if scene.Narration != "" {
				s.Prompt += "\n旁白：" + scene.Narration
			}
			for _, u := range scene.Utterances {
				name := u.SpeakerID
				for _, e := range p.EntityCatalog.Entities {
					if e.ID == u.SpeakerID && e.Revision == u.SpeakerRevision {
						name = e.Name
						break
					}
				}
				s.Prompt += fmt.Sprintf("\n%s说：%s", name, u.Text)
			}
			s.Utterances = scene.Utterances
			if len([]rune(s.Prompt)) > 5000 {
				return s, errors.New("视频描述与素材描述合计最多5000字，请精简后生成")
			}
		}
	} else if in.Path == "first_frame_video" {
		s.VideoInputMode = "first_frame"
		for _, take := range d.MediaTakes {
			if take.ID == d.SelectedMedia[scene.ID+":image"] && (take.Stale || take.DependencyHash != dependencyHash(p.EntityCatalog, scene, "reference_image_then_video", "")) {
				return s, errors.New("所选首帧基于旧设定，请重新生成并选用分镜图")
			}
		}
		if !m.Supports(provider.Video) || !cap.FirstFrame {
			return s, errors.New("model does not support first-frame video")
		}
		if s.DurationSeconds == 0 {
			s.DurationSeconds = scene.DurationSeconds
		}
		validDuration := false
		for _, n := range cap.Durations {
			if n == s.DurationSeconds {
				validDuration = true
			}
		}
		if !validDuration {
			return s, errors.New("unsupported video duration")
		}
		if s.Resolution == "" {
			s.Resolution = "720P"
		}
		validResolution := false
		for _, r := range cap.Resolutions {
			if r == s.Resolution {
				validResolution = true
			}
		}
		if !validResolution {
			return s, errors.New("unsupported resolution")
		}
		for _, b := range d.Bindings {
			if b.SceneID == scene.ID && b.Usage == "first_frame" {
				r, err := c.reference(ctx, b.MaterialID, "first_frame")
				if err != nil {
					return s, err
				}
				s.FirstFrameMaterialID = r.MaterialID
				s.Entities = append(s.Entities, task.SnapshotEntity{BindingID: "first_frame", EntityID: "first_frame", References: []task.MediaReference{r}})
			}
		}
		if len(s.Entities) != 1 {
			return s, errors.New("请先生成并选用一张分镜图作为视频首帧")
		}
	} else {
		return s, errors.New("unsupported generation path; use multi-image first frame then video")
	}
	return task.FreezeSnapshot(s)
}

func dependencyHash(c project.EntityCatalog, scene story.Scene, path, utteranceID string) string {
	r, err := ResolveBindings(c, scene, false)
	if err != nil {
		return "invalid"
	}
	var value any
	if path == "speech" {
		for _, u := range scene.Utterances {
			if u.ID == utteranceID {
				u.Order = 0
				u.PauseAfterMS = 0
				value = struct {
					Line  story.Utterance
					Voice project.VoiceProfile
				}{u, r.Voices[u.ID]}
			}
		}
	} else {
		value = struct {
			Prompt   string
			Entities []ResolvedBinding
		}{r.Prompt, r.Entities}
	}
	b, _ := json.Marshal(value)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (c GenerationCompiler) reference(ctx context.Context, id, label string) (task.MediaReference, error) {
	r := task.MediaReference{MaterialID: id, Label: label}
	a, err := c.Materials.Get(ctx, id)
	if err != nil {
		return r, err
	}
	name := strings.TrimPrefix(a.URL, "/media/")
	if name == a.URL || name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\`) {
		return r, errors.New("reference must be an uploaded local material")
	}
	info, err := os.Stat(filepath.Join(c.Directory, name))
	if err != nil {
		return r, err
	}
	limit := c.MaxReferenceBytes
	if limit <= 0 {
		limit = provider.MaxLocalImageBytes
	}
	if info.Size() == 0 || info.Size() > limit {
		return r, fmt.Errorf("reference image exceeds configured limit %d bytes", limit)
	}
	b, err := os.ReadFile(filepath.Join(c.Directory, name))
	if err != nil {
		return r, err
	}
	hash := sha256.Sum256(b)
	r.ContentHash = hex.EncodeToString(hash[:])
	r.URL = a.URL
	return r, nil
}

func (s TaskService) compileShot(g *gin.Context, in GenerationRequest) (task.GenerationSnapshot, error) {
	p, err := s.Projects.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		return task.GenerationSnapshot{}, err
	}
	d, err := draftByID(&p, g.Param("draftId"))
	if err != nil {
		return task.GenerationSnapshot{}, err
	}
	for _, scene := range d.Story.Scenes {
		if scene.ID == g.Param("sceneId") {
			return (GenerationCompiler{Models: s.Providers.Catalog.Models, Materials: s.Materials, Directory: s.UploadDir}).Compile(g.Request.Context(), p, *d, scene, in)
		}
	}
	return task.GenerationSnapshot{}, errors.New("scene not found")
}

func (s TaskService) GenerationPreview(g *gin.Context) {
	var in GenerationRequest
	if err := g.ShouldBindJSON(&in); err != nil {
		writeError(g, Invalid("invalid_generation_input", err))
		return
	}
	snap, err := s.compileShot(g, in)
	if errors.Is(err, project.ErrVersionConflict) {
		writeError(g, Conflict("draft_version_conflict", err))
		return
	}
	if err != nil {
		g.JSON(422, gin.H{"valid": false, "issues": []gin.H{{"field": "generation", "code": "input_invalid", "message": err.Error()}}})
		return
	}
	g.JSON(200, gin.H{"valid": true, "issues": []any{}, "snapshot": snap})
}
