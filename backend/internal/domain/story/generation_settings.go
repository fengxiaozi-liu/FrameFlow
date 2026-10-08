package story

import "errors"

type GenerationSettings struct {
	ImageModelID    string `json:"image_model_id,omitempty"`
	VideoModelID    string `json:"video_model_id"`
	AspectRatio     string `json:"aspect_ratio"`
	Resolution      string `json:"resolution"`
	DurationSeconds int    `json:"duration_seconds"`
}

func (s GenerationSettings) Validate() error {
	if s.VideoModelID == "" || s.DurationSeconds < 1 || s.Resolution == "" {
		return errors.New("请选择视频模型、清晰度与时长")
	}
	if s.AspectRatio != "16:9" && s.AspectRatio != "9:16" && s.AspectRatio != "1:1" {
		return errors.New("不支持的画幅")
	}
	return nil
}
