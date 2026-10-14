package story

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

type GenerationSettings struct {
	VideoInputMode       string `json:"video_input_mode,omitempty"`
	FirstFrameMaterialID string `json:"first_frame_material_id,omitempty"`
	LastFrameMaterialID  string `json:"last_frame_material_id,omitempty"`
	ImageModelID         string `json:"image_model_id,omitempty"`
	VideoModelID         string `json:"video_model_id"`
	AspectRatio          string `json:"aspect_ratio"`
	Resolution           string `json:"resolution"`
	DurationSeconds      int    `json:"duration_seconds"`
}

func (s GenerationSettings) Validate() error {
	if s.VideoInputMode != "" && s.VideoInputMode != "first_frame" && s.VideoInputMode != "reference_images" {
		return errors.New("不支持的视频生成模式")
	}
	// Drafts may retain an incomplete frame selection; submission validates completeness.

	if s.VideoModelID == "" || s.DurationSeconds < 1 || s.Resolution == "" {
		return errors.New("请选择视频模型、清晰度与时长")
	}
	if err := provider.ValidateRatioValue(s.AspectRatio); err != nil {
		return err
	}
	return nil
}
