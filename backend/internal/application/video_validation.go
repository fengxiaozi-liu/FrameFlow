package application

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/gin-gonic/gin"
)

type VideoValidationIssue struct {
	SceneID string `json:"scene_id"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (s TaskService) ValidateSceneVideo(g *gin.Context) {
	var input struct {
		ModelID    string `json:"model_id"`
		Resolution string `json:"resolution"`
	}
	if err := g.ShouldBindJSON(&input); err != nil {
		writeError(g, Invalid("invalid_video_validation", err))
		return
	}
	issues := s.validateVideoScene(g, g.Param("id"), g.Param("draftId"), g.Param("sceneId"), input.ModelID, input.Resolution)
	g.JSON(http.StatusOK, gin.H{"valid": len(issues) == 0, "issues": issues})
}

func (s TaskService) validateVideoScene(g *gin.Context, projectID, draftID, sceneID, modelID, resolution string) []VideoValidationIssue {
	issues := []VideoValidationIssue{}
	add := func(field, message string) {
		issues = append(issues, VideoValidationIssue{SceneID: sceneID, Field: field, Message: message})
	}
	if s.Projects == nil || s.Materials == nil || s.Providers.Catalog.Models == nil {
		add("configuration", "视频服务尚未配置完整")
		return issues
	}
	model, err := s.Providers.Catalog.Models.GetModel(g.Request.Context(), modelID)
	if err != nil || !model.Supports(provider.Video) {
		add("model_id", "请选择可用的视频模型")
		return issues
	}
	capability, known := provider.VideoCapabilityFor(model.RemoteID)
	if !known {
		add("model_id", "该模型的输入能力尚未确认")
		return issues
	}
	p, err := s.Projects.Get(g.Request.Context(), projectID)
	if err != nil {
		add("project_id", "项目不存在")
		return issues
	}
	var d *project.Draft
	for i := range p.Drafts {
		if p.Drafts[i].ID == draftID {
			d = &p.Drafts[i]
			break
		}
	}
	if d == nil {
		add("draft_id", "草稿不存在")
		return issues
	}
	var duration int
	var visualPrompt string
	for _, scene := range d.Story.Scenes {
		if scene.ID == sceneID {
			duration = scene.DurationSeconds
			visualPrompt = scene.VisualPrompt
			break
		}
	}
	if duration == 0 {
		add("scene_id", "镜头不存在或时长无效")
		return issues
	}
	if strings.TrimSpace(visualPrompt)=="" {add("visual_prompt","画面描述不能为空")}
	usages := []string{}
	for _, binding := range d.Bindings {
		if binding.SceneID != sceneID {
			continue
		}
		if binding.Usage != "voiceover" {
			usages = append(usages, binding.Usage)
		}
		if binding.Usage == "character_reference" || binding.Usage == "scene_reference" || binding.Usage == "prop_reference" {
			add(binding.Usage, "当前模型不支持参考图输入，请解绑或更换模型")
			continue
		}
		asset, err := s.Materials.Get(g.Request.Context(), binding.MaterialID)
		if err != nil {
			add(binding.Usage, "绑定的素材不存在")
			continue
		}
		if err := validateBinding(binding, asset); err != nil {
			add(binding.Usage, err.Error())
			continue
		}
		if asset.Media.SizeBytes <= 0 {
			add(binding.Usage, "素材缺少真实文件大小")
			continue
		}
		if binding.Usage == "first_frame" || binding.Usage == "last_frame" {
			if asset.Media.Width < 240 || asset.Media.Width > 8000 || asset.Media.Height < 240 || asset.Media.Height > 8000 {
				add(binding.Usage, "图片宽高需在 240–8000 像素之间")
			}
			if asset.Media.Width > 0 && asset.Media.Height > 0 &&
				(asset.Media.Width > 8*asset.Media.Height || asset.Media.Height > 8*asset.Media.Width) {
				add(binding.Usage, "图片宽高比需在 1:8–8:1 之间")
			}
		}
		if binding.Usage == "driving_audio" {
			if asset.Media.SizeBytes > capability.MaxAudioBytes || asset.Media.DurationSeconds < float64(capability.MinAudioDuration) || asset.Media.DurationSeconds > float64(capability.MaxAudioDuration) {
				add(binding.Usage, "驱动音频需为 2–30 秒且不超过 15 MB")
			}
			if !strings.HasPrefix(asset.URL, "/media/") && !strings.HasPrefix(asset.URL, "https://") && !strings.HasPrefix(asset.URL, "oss://") {
				add(binding.Usage, "音频需要可交付的对象存储 URL")
			}
			if strings.HasPrefix(asset.URL, "/media/") &&
				(os.Getenv("FRAMEFLOW_AUDIO_OBJECT_BASE_URL") == "" || os.Getenv("FRAMEFLOW_AUDIO_OBJECT_BUCKET") == "" ||
					os.Getenv("FRAMEFLOW_AUDIO_OBJECT_ACCESS_KEY_ID") == "" || os.Getenv("FRAMEFLOW_AUDIO_OBJECT_ACCESS_KEY_SECRET") == "") {
				add(binding.Usage, "未配置音频对象存储，无法提交驱动音频")
			}
		} else if asset.Media.SizeBytes > capability.MaxImageBytes {
			add(binding.Usage, "图片不得超过 20 MB")
		}
		if strings.HasPrefix(asset.URL, "/media/") {
			name := strings.TrimPrefix(asset.URL, "/media/")
			if name == "" || filepath.Base(name) != name || s.UploadDir == "" {
				add(binding.Usage, "本地素材路径无效")
				continue
			}
			if _, err := os.Stat(filepath.Join(s.UploadDir, name)); err != nil {
				add(binding.Usage, "素材文件不存在")
			}
		}
	}
	if resolution == "" {
		resolution = "1080P"
	}
	if err := capability.ValidateInput(duration, resolution, usages); err != nil {
		field := "media"
		if duration < capability.MinDuration || duration > capability.MaxDuration {
			field = "duration_seconds"
		}
		if resolution != "720P" && resolution != "1080P" {
			field = "resolution"
		}
		add(field, fmt.Sprintf("%v", err))
	}
	return issues
}

var errVideoValidation = errors.New("video scene validation failed")
