package provider

import (
	"context"
	"errors"

	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

// GenerationProcessor is the worker adapter that invokes the configured
// provider port while keeping queue and transport concerns outside the domain.
type GenerationProcessor struct {
	Configs domain.Repository
	Story   domain.StoryPort
	Image   domain.ImagePort
	Video   domain.VideoPort
}

func NewGenerationProcessor(configs domain.Repository) GenerationProcessor {
	return GenerationProcessor{
		Configs: configs,
		Story:   StoryMock{},
		Image:   ImageMock{},
		Video:   VideoMock{},
	}
}

func (p GenerationProcessor) Process(ctx context.Context, item task.Task, progress func(int, string)) error {
	config, ok := p.Configs.Get(item.ProviderCode)
	if !ok {
		return errors.New("provider not found")
	}
	if !config.Enabled || config.Status != domain.Healthy {
		return errors.New("provider is not ready")
	}
	progress(20, "preparing")
	options := domain.RequestOptions{}
	switch item.Kind {
	case "story":
		progress(55, "generating_story")
		_, err := p.Story.Generate(ctx, domain.StoryRequest{Model: config.Model}, options)
		return err
	case "image":
		progress(55, "generating_image")
		_, err := p.Image.Generate(ctx, domain.ImageRequest{Model: config.Model}, options)
		return err
	case "video":
		progress(55, "rendering_video")
		_, err := p.Video.Generate(ctx, domain.VideoRequest{Model: config.Model}, options)
		if err == nil {
			progress(85, "composing")
		}
		return err
	default:
		return errors.New("unsupported task kind")
	}
}
