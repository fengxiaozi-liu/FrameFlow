package application

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/gin-gonic/gin"
	"path/filepath"
	"strings"
	"time"
)

func (s ProjectService) WriteShotContext(g *gin.Context) {
	var in struct {
		Expected   *int64                    `json:"expected_version"`
		Bindings   []story.EntityBinding     `json:"entity_bindings"`
		Prompt     story.PromptDocument      `json:"prompt_document"`
		Utterances []story.Utterance         `json:"utterances"`
		Title      *string                   `json:"title"`
		Narration  *string                   `json:"narration"`
		Settings   *story.GenerationSettings `json:"generation_settings"`
	}
	if err := g.ShouldBindJSON(&in); err != nil || in.Expected == nil {
		writeError(g, Invalid("invalid_context", errors.New("expected_version required")))
		return
	}
	for _, binding := range in.Bindings {
		if binding.ReferenceMaterialIDs == nil {
			continue
		}
		for _, id := range *binding.ReferenceMaterialIDs {
			asset, err := s.Materials.Get(g.Request.Context(), id)
			ext := strings.ToLower(filepath.Ext(asset.URL))
			if err != nil || !strings.HasPrefix(asset.URL, "/media/") || (ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp") {
				writeError(g, Invalid("invalid_context", errors.New("镜头参考图不存在或不是已上传图片")))
				return
			}
		}
	}
	if in.Settings != nil {
		if err := in.Settings.Validate(); err != nil {
			writeError(g, Invalid("invalid_generation_settings", err))
			return
		}
		if in.Settings.ImageModelID != "" {
			image, e := s.Models.GetModel(g.Request.Context(), in.Settings.ImageModelID)
			ic, known := image.InputCapability()
			if e != nil || !known || !image.Enabled || !image.Supported || !image.Supports(provider.Image) || ic.ReferenceImages == 0 {
				writeError(g, Invalid("invalid_generation_settings", errors.New("分镜图模型不可用")))
				return
			}
		}
		video, e := s.Models.GetModel(g.Request.Context(), in.Settings.VideoModelID)
		vc, known := video.InputCapability()
		if in.Settings.VideoInputMode != "" {
			if _, modeErr := video.Operations[provider.Video].SelectVideoMode(in.Settings.VideoInputMode); modeErr != nil {
				writeError(g, Invalid("invalid_generation_settings", modeErr))
				return
			}
		}
		validDuration, validResolution := false, false
		for _, v := range vc.Durations {
			validDuration = validDuration || v == in.Settings.DurationSeconds
		}
		for _, v := range vc.Resolutions {
			validResolution = validResolution || v == in.Settings.Resolution
		}
		if e != nil || !known || !video.Enabled || !video.Supported || !video.Supports(provider.Video) || (!vc.FirstFrame && !vc.ReferenceVideo) || !validDuration || !validResolution {
			writeError(g, Invalid("invalid_generation_settings", errors.New("视频模型、时长或清晰度不可用")))
			return
		}
		if err := video.Operations[provider.Video].ValidateAspectRatio(in.Settings.AspectRatio); err != nil {
			writeError(g, Invalid("invalid_generation_settings", err))
			return
		}
	}
	atomic, ok := s.Repo.(project.AtomicRepository)
	if !ok {
		writeError(g, errors.New("atomic repository required"))
		return
	}
	var result project.Draft
	_, err := atomic.Update(g.Request.Context(), g.Param("id"), func(p *project.Project) error {
		d, err := draftByID(p, g.Param("draftId"))
		if err != nil {
			return err
		}
		result = *d
		if d.Version != *in.Expected {
			return project.ErrVersionConflict
		}
		for i := range d.Story.Scenes {
			scene := &d.Story.Scenes[i]
			if scene.ID != g.Param("sceneId") {
				continue
			}
			next := *scene
			if in.Title != nil {
				if strings.TrimSpace(*in.Title) == "" {
					return errors.New("请填写分镜名称")
				}
				if len([]rune(*in.Title)) > 100 {
					return errors.New("镜头标题最多100字")
				}
				next.Title = *in.Title
			}
			if in.Narration != nil {
				next.Narration = *in.Narration
			}
			next.EntityBindings = in.Bindings
			next.PromptDocument = &in.Prompt
			next.MergeDescriptionSupplements()
			next.Utterances = in.Utterances
			if in.Settings != nil {
				if err := in.Settings.Validate(); err != nil {
					return err
				}
				next.GenerationSettings = in.Settings
				next.DurationSeconds = in.Settings.DurationSeconds
			}
			r, err := ResolveBindings(p.EntityCatalog, next, false)
			if err != nil {
				return err
			}
			next.VisualPrompt = r.Prompt
			*scene = next
			d.Version++
			d.UpdatedAt = time.Now().UTC()
			for i := range d.MediaTakes {
				if d.MediaTakes[i].SceneID == scene.ID {
					path := "reference_image_then_video"
					if d.MediaTakes[i].Kind == "speech" {
						path = "speech"
					}
					d.MediaTakes[i].Stale = d.MediaTakes[i].DependencyHash != dependencyHash(p.EntityCatalog, next, path, d.MediaTakes[i].UtteranceID)
				}
			}
			result = *d
			return nil
		}
		return errors.New("scene not found")
	})
	if errors.Is(err, project.ErrVersionConflict) {
		g.JSON(409, gin.H{"code": "draft_version_conflict", "message": "草稿已改变，本地内容仍保留", "draft": result})
		return
	}
	if err != nil {
		writeError(g, Invalid("invalid_context", err))
		return
	}
	g.JSON(200, result)
}
