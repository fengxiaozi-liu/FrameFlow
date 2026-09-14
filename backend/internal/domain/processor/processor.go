package processor

import (
	"context"
	"errors"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type Worker interface {
	Enqueue(task.Task)
	Run(context.Context, func(context.Context, task.Task, func(int, string)) error)
}

// Processor 通过已注册的 provider 执行队列中的生成任务。
// 它只依赖领域契约，具体厂商客户端由组合根通过 provider.Registry 注入。
type Processor struct {
	Configs  provider.Repository
	Registry *provider.Registry
	Worker   Worker
}

func New(configs provider.Repository, registry *provider.Registry, worker Worker) *Processor {
	return &Processor{Configs: configs, Registry: registry, Worker: worker}
}

func (p *Processor) Enqueue(item task.Task) {
	p.Worker.Enqueue(item)
}

func (p *Processor) Start(ctx context.Context) {
	p.Worker.Run(ctx, p.Process)
}

func (p *Processor) Process(ctx context.Context, item task.Task, progress func(int, string)) error {
	config, ok := p.Configs.Get(item.ProviderCode)
	if !ok && item.ProviderCode == "" {
		config = provider.Config{Code: "default-" + string(item.Kind), Vendor: "mock", Capability: capability(item.Kind), Model: "mock", Enabled: true, Status: provider.Healthy}
		ok = true
	}
	if !ok {
		return errors.New("provider not found")
	}
	if !config.Enabled || config.Status != provider.Healthy {
		return errors.New("provider is not ready")
	}
	client, err := p.Registry.Resolve(config)
	if err != nil {
		return err
	}
	progress(20, "preparing")
	options := provider.RequestOptions{}
	switch item.Kind {
	case task.KindStory:
		generator, ok := client.(provider.StoryGenerator)
		if !ok {
			return errors.New("provider does not support story generation")
		}
		progress(55, "generating_story")
		_, err = generator.Generate(ctx, provider.StoryRequest{Prompt: item.Input.Prompt, Model: config.Model}, options)
	case task.KindImage:
		generator, ok := client.(provider.ImageGenerator)
		if !ok {
			return errors.New("provider does not support image generation")
		}
		progress(55, "generating_image")
		_, err = generator.Generate(ctx, provider.ImageRequest{Prompt: item.Input.Prompt, Model: config.Model, AspectRatio: item.Input.AspectRatio}, options)
	case task.KindVideo:
		generator, ok := client.(provider.VideoGenerator)
		if !ok {
			return errors.New("provider does not support video generation")
		}
		progress(55, "rendering_video")
		_, err = generator.Generate(ctx, provider.VideoRequest{Prompt: item.Input.Prompt, Model: config.Model, AspectRatio: item.Input.AspectRatio, SourceImageURL: item.Input.SourceImageURL}, options)
		if err == nil {
			progress(85, "composing")
		}
	default:
		return errors.New("unsupported task kind")
	}
	return err
}

func capability(kind task.Kind) provider.Capability {
	switch kind {
	case task.KindStory:
		return provider.Story
	case task.KindImage:
		return provider.Image
	case task.KindVideo:
		return provider.Video
	default:
		return ""
	}
}
