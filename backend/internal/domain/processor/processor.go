package processor

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/websocket"
	"log"
	"sync"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type Worker interface {
	Enqueue(context.Context, task.Task) error
	Run(context.Context, func(context.Context, task.Task))
}

// Processor 通过已注册的 provider 执行队列中的生成任务。
// 具体厂商客户端由组合根通过 provider.Registry 注入。
type Processor struct {
	Configs     provider.Repository
	Registry    *provider.Registry
	Worker      Worker
	Task        task.Repository
	Connections *websocket.Hub
	mu          sync.Mutex
	running     map[string]context.CancelFunc
}

func New(configs provider.Repository, registry *provider.Registry, worker Worker, store task.Repository) *Processor {
	return &Processor{
		Configs:     configs,
		Registry:    registry,
		Worker:      worker,
		Task:        store,
		Connections: websocket.Connections,
	}
}

func (p *Processor) Enqueue(ctx context.Context, item task.Task) error {
	return p.Worker.Enqueue(ctx, item)
}

// CancelTask cancels provider I/O for a running task. Queued tasks are skipped
// by Execute after their persisted state has been changed to cancelled.
func (p *Processor) CancelTask(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if cancel := p.running[id]; cancel != nil {
		cancel()
	}
	return nil
}

func (p *Processor) Start(ctx context.Context) {
	p.Worker.Run(ctx, p.Execute)
}

func (p *Processor) Process(ctx context.Context, item task.Task, progress func(int, task.Stage)) error {
	config, err := p.Configs.Get(ctx, item.ProviderCode)
	if errors.Is(err, fault.ErrNotFound) && item.ProviderCode == "" {
		config = provider.Config{
			Code:       "default-" + string(item.Kind),
			Vendor:     "mock",
			Capability: capability(item.Kind),
			Model:      "mock",
			Enabled:    true,
			Status:     provider.Healthy,
		}
		err = nil
	}
	if err != nil {
		return err
	}
	if !config.Enabled || config.Status != provider.Healthy {
		return errors.New("provider is not ready")
	}
	client, err := p.Registry.Resolve(ctx, config)
	if err != nil {
		return err
	}
	progress(20, task.StagePreparing)
	options := provider.RequestOptions{}
	switch item.Kind {
	case task.KindStory:
		generator, ok := client.(provider.StoryGenerator)
		if !ok {
			return errors.New("provider does not support story generation")
		}
		progress(55, task.StageGeneratingStory)
		_, err = generator.Generate(ctx, provider.StoryRequest{
			Prompt: item.Input.Prompt,
			Model:  config.Model,
		}, options)
	case task.KindImage:
		generator, ok := client.(provider.ImageGenerator)
		if !ok {
			return errors.New("provider does not support image generation")
		}
		progress(55, task.StageGeneratingImage)
		_, err = generator.Generate(ctx, provider.ImageRequest{
			Prompt:      item.Input.Prompt,
			Model:       config.Model,
			AspectRatio: item.Input.AspectRatio,
		}, options)
	case task.KindVideo:
		generator, ok := client.(provider.VideoGenerator)
		if !ok {
			return errors.New("provider does not support video generation")
		}
		progress(55, task.StageRenderingVideo)
		_, err = generator.Generate(ctx, provider.VideoRequest{
			Prompt:         item.Input.Prompt,
			Model:          config.Model,
			AspectRatio:    item.Input.AspectRatio,
			SourceImageURL: item.Input.SourceImageURL,
		}, options)
		if err == nil {
			progress(85, task.StageComposing)
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

// Execute owns task state, persistence, progress events and retry policy.
func (p *Processor) Execute(ctx context.Context, item task.Task) {
	ctx, cancel := context.WithCancel(ctx)
	p.mu.Lock()
	if p.running == nil {
		p.running = make(map[string]context.CancelFunc)
	}
	p.running[item.ID] = cancel
	p.mu.Unlock()
	defer func() {
		cancel()
		p.mu.Lock()
		delete(p.running, item.ID)
		p.mu.Unlock()
	}()
	current, getErr := p.Task.Get(ctx, item.ID)
	if getErr != nil || current.Status != task.StatusQueued || ctx.Err() != nil {
		return
	}
	item = current
	item.Start(time.Now().UTC())
	item.Stage = task.StagePreparing
	if err := p.Task.Save(ctx, item); err != nil {
		log.Printf("save task %s: %v", item.ID, err)
		return
	}
	p.Connections.Broadcast(ctx, task.Event{
		TaskID:   item.ID,
		Status:   item.Status,
		Progress: item.Progress,
		Stage:    item.Stage,
		At:       item.UpdatedAt,
	})
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if !p.active(ctx, item) || ctx.Err() != nil {
			return
		}
		saved := true
		err = p.Process(ctx, item, func(progress int, stage task.Stage) {
			if !saved || !p.active(ctx, item) || ctx.Err() != nil {
				return
			}
			item.Advance(progress, stage, time.Now().UTC())
			if err := p.Task.Save(ctx, item); err != nil {
				log.Printf("save task %s: %v", item.ID, err)
				saved = false
				return
			}
			p.Connections.Broadcast(ctx, task.Event{
				TaskID:   item.ID,
				Status:   item.Status,
				Progress: item.Progress,
				Stage:    item.Stage,
				At:       item.UpdatedAt,
			})
		})
		if !saved || !p.active(ctx, item) || ctx.Err() != nil {
			return
		}
		if err == nil {
			break
		}
		item.RetryCount = attempt + 1
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(1<<attempt) * 25 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
	if err != nil {
		item.Fail(err.Error(), time.Now().UTC())
	} else {
		item.Succeed(time.Now().UTC())
	}
	if err := p.Task.Save(ctx, item); err != nil {
		log.Printf("save task %s: %v", item.ID, err)
		return
	}
	p.Connections.Broadcast(ctx, task.Event{
		TaskID:   item.ID,
		Status:   item.Status,
		Progress: item.Progress,
		Stage:    item.Stage,
		At:       item.UpdatedAt,
	})
}

func (p *Processor) active(ctx context.Context, item task.Task) bool {
	current, getErr := p.Task.Get(ctx, item.ID)
	return getErr == nil && current.Status == task.StatusRunning
}
