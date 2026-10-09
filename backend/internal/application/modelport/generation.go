package modelport

import (
	"context"
	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

// Operation ports are deliberately separate: a new image adapter need not
// implement text, speech or video stubs.
type TextGenerator interface {
	GenerateText(context.Context, domain.Connection, domain.Model, domain.StoryRequest, domain.RequestOptions) (domain.StoryResult, error)
}
type ImageGenerator interface {
	GenerateImage(context.Context, domain.Connection, domain.Model, domain.ImageRequest, domain.RequestOptions) (domain.ImageResult, error)
}
type VideoGenerator interface {
	GenerateVideo(context.Context, domain.Connection, domain.Model, domain.VideoRequest, domain.RequestOptions) (domain.VideoResult, error)
	PollVideo(context.Context, domain.Connection, domain.Model, string) (domain.VideoResult, bool, error)
}
type AudioUploader interface {
	UploadAudio(context.Context, domain.Connection, domain.Model, string) (string, error)
}
