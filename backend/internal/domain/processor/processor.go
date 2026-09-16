package processor

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
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
// 它只依赖领域契约，具体厂商客户端由组合根通过 provider.Registry 注入。
type Processor struct {
	Configs   provider.Repository
	Registry  *provider.Registry
	Worker    Worker
	Store     task.Repository
	Events    task.EventRepository
	SendEvent task.Sender
	mu        sync.Mutex
	running   map[string]context.CancelFunc
}

func New(configs provider.Repository, registry *provider.Registry, worker Worker, store task.Repository) *Processor {
	return &Processor{Configs: configs, Registry: registry, Worker: worker, Store: store}
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

func (p *Processor) Process(ctx context.Context, item task.Task, progress func(int, string)) error {
	config, err := p.Configs.Get(ctx, item.ProviderCode)
	if errors.Is(err, fault.ErrNotFound) && item.ProviderCode == "" {
		config = provider.Config{Code: "default-" + string(item.Kind), Vendor: "mock", Capability: capability(item.Kind), Model: "mock", Enabled: true, Status: provider.Healthy}
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
	current, getErr := p.Store.Get(ctx, item.ID)
	if getErr != nil || current.Status != task.StatusQueued || ctx.Err() != nil {
		return
	}
	item = current
	item.Start(time.Now().UTC())
	item.Stage = "preparing"
	if !p.save(ctx, item) {
		return
	}
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if !p.active(ctx, item) || ctx.Err() != nil {
			return
		}
		saved := true
		err = p.Process(ctx, item, func(progress int, stage string) {
			if !saved || !p.active(ctx, item) || ctx.Err() != nil {
				return
			}
			item.Advance(progress, stage, time.Now().UTC())
			saved = p.save(ctx, item)
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
	p.save(ctx, item)
}

func (p *Processor) active(ctx context.Context, item task.Task) bool {
	current, getErr := p.Store.Get(ctx, item.ID)
	return getErr == nil && current.Status == task.StatusRunning
}

func (p *Processor) save(ctx context.Context, item task.Task) bool {
	if err := p.Store.Save(ctx, item); err != nil {
		log.Printf("save task %s: %v", item.ID, err)
		return false
	}
	events := p.Events
	if events == nil {
		events, _ = p.Store.(task.EventRepository)
	}
	if err := task.Publish(ctx, events, p.SendEvent, item.SessionID, task.Event{TaskID: item.ID, Status: item.Status, Progress: item.Progress, Stage: item.Stage, At: item.UpdatedAt}); err != nil {
		log.Printf("publish task event %s: %v", item.ID, err)
		return false
	}
	return true
}
