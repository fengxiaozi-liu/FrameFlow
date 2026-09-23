package processor

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/websocket"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type Worker interface {
	Enqueue(context.Context, task.Task) error
	Run(context.Context, func(context.Context, task.Task))
}

// Processor uses the configured model client to execute queued tasks.
type Processor struct {
	Models              provider.ModelRepository
	ProviderConnections provider.ConnectionRepository
	ModelClient         provider.ModelClient
	SaveResult          func(context.Context, string, task.Kind, string) (string, error)
	Compose             func(context.Context, string, task.CompositionInput, func(int, task.Stage)) (string, float64, error)
	MediaDir            string
	Worker              Worker
	Task                task.Repository
	Connections         *websocket.Hub
	Results             interface {
		Apply(context.Context, task.Task) error
	}
	mu      sync.Mutex
	running map[string]context.CancelFunc
}

func New(worker Worker, store task.Repository) *Processor {
	return &Processor{
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

func (p *Processor) processModel(ctx context.Context, item *task.Task, progress func(int, task.Stage)) error {
	if item.Kind == task.KindComposition {
		if item.ResultURL != "" && strings.HasPrefix(item.ResultURL, "/media/") {
			return nil
		}
		if item.Composition == nil || p.Compose == nil {
			return errors.New("composition engine is not configured")
		}
		result, duration, err := p.Compose(ctx, item.ID, *item.Composition, progress)
		if err != nil {
			return err
		}
		item.ResultURL, item.ResultDurationSeconds = result, duration
		return p.Task.Save(ctx, *item)
	}
	if item.Kind == task.KindStory && (item.ResultText != "" || item.ResultCandidate != nil) || item.ResultURL != "" && (item.Kind == task.KindImage || item.Kind == task.KindVideo) && strings.HasPrefix(item.ResultURL, "/media/") {
		return nil
	}
	if p.Models == nil || p.ProviderConnections == nil || p.ModelClient == nil {
		return errors.New("model execution is not configured")
	}
	model, err := p.Models.GetModel(ctx, item.ModelID)
	if err != nil {
		return err
	}
	connection, err := p.ProviderConnections.GetConnection(ctx, model.ConnectionID)
	if err != nil {
		return err
	}
	allowed := false
	for _, cap := range model.Capabilities {
		if cap == capability(item.Kind) {
			allowed = true
			break
		}
	}
	if !model.Supported || !allowed {
		return provider.ErrUnsupported
	}
	progress(20, task.StagePreparing)
	options := provider.RequestOptions{Timeout: 5 * time.Minute}
	switch item.Kind {
	case task.KindStory:
		progress(55, task.StageGeneratingStory)
		request := provider.StoryRequest{Prompt: item.Input.Prompt, Model: model.RemoteID}
		if item.Target != "" {
			version, systemPrompt, userPrompt, buildErr := story.BuildGenerationPrompt(story.CandidateTarget(item.Target), item.Input.Prompt, item.Input.SourceBody)
			if buildErr != nil {
				return buildErr
			}
			request.SystemPrompt, request.Prompt = systemPrompt, userPrompt
			item.CapabilityVersion = version
		}
		result, err := p.ModelClient.GenerateText(ctx, connection, model, request, options)
		if err != nil {
			return err
		}
		if item.Target == "" {
			item.ResultText = result.Document // Legacy task compatibility.
		} else {
			candidate, parseErr := story.ParseCandidateResult(story.CandidateTarget(item.Target), result.Document)
			if parseErr != nil {
				return parseErr
			}
			candidate.ID = item.ID
			candidate.TaskID = item.ID
			candidate.SourceVersion = item.InputVersion
			candidate.SourceBody = item.Input.SourceBody
			candidate.Mode = item.Input.Mode
			candidate.Instruction = item.Input.Prompt
			candidate.PromptVersion = item.CapabilityVersion
			candidate.CreatedAt = item.CreatedAt
			item.ResultCandidate = &candidate
		}
	case task.KindImage:
		progress(55, task.StageGeneratingImage)
		result, err := p.ModelClient.GenerateImage(ctx, connection, model, provider.ImageRequest{Prompt: item.Input.Prompt, Model: model.RemoteID, AspectRatio: item.Input.AspectRatio}, options)
		if err != nil {
			return err
		}
		item.ResultURL = result.URL
	case task.KindVideo:
		progress(55, task.StageRenderingVideo)
		pollCtx, endPoll := context.WithTimeout(ctx, 20*time.Minute)
		defer endPoll()
		if item.RemoteTaskID == "" {
			source, err := p.videoSource(item.Input.SourceImageURL)
			if err != nil {
				return err
			}
			lastFrame := item.Input.LastFrameURL
			if lastFrame != "" {
				lastFrame, err = p.videoSource(lastFrame)
				if err != nil {
					return err
				}
			}
			audio := item.Input.DrivingAudioURL
			if strings.HasPrefix(audio, "/media/") {
				if p.MediaDir == "" {
					return errors.New("local audio delivery is not configured")
				}
				name := strings.TrimPrefix(audio, "/media/")
				if name == "" || filepath.Base(name) != name {
					return errors.New("invalid local driving audio")
				}
				upload, ok := p.ModelClient.(interface {
					UploadAudio(context.Context, provider.Connection, provider.Model, string) (string, error)
				})
				if !ok {
					return errors.New("object storage audio delivery is not configured")
				}
				audio, err = upload.UploadAudio(ctx, connection, model, filepath.Join(p.MediaDir, name))
				if err != nil {
					return err
				}
			}
			result, err := p.ModelClient.GenerateVideo(ctx, connection, model, provider.VideoRequest{Prompt: item.Input.Prompt, Model: model.RemoteID, SourceImageURL: source, LastFrameURL: lastFrame, DrivingAudioURL: audio, Duration: item.Input.Duration, Resolution: item.Input.Resolution, AspectRatio: item.Input.AspectRatio}, options)
			if err != nil {
				return err
			}
			if result.URL != "" {
				item.ResultURL = result.URL
				break
			}
			if result.JobReference == "" {
				return errors.New("video model returned neither result nor task id")
			}
			item.RemoteTaskID = result.JobReference
			if p.Task == nil {
				return errors.New("task repository required for async video")
			}
			if err := p.Task.Save(ctx, *item); err != nil {
				return err
			}
		}
		for {
			result, done, err := p.ModelClient.PollVideo(pollCtx, connection, model, item.RemoteTaskID)
			if err != nil {
				return err
			}
			if done {
				item.ResultURL = result.URL
				progress(85, task.StageComposing)
				break
			}
			select {
			case <-pollCtx.Done():
				return pollCtx.Err()
			case <-time.After(15 * time.Second):
			}
		}
	default:
		return provider.ErrUnsupported
	}
	if item.ResultURL != "" && p.SaveResult != nil {
		path, err := p.SaveResult(ctx, item.ID, item.Kind, item.ResultURL)
		if err != nil {
			return err
		}
		item.ResultURL = path
	}
	if p.Task != nil {
		return p.Task.Save(ctx, *item)
	}
	return nil
}

func (p *Processor) videoSource(source string) (string, error) {
	if !strings.HasPrefix(source, "/media/") {
		return source, nil
	}
	name := strings.TrimPrefix(source, "/media/")
	if p.MediaDir == "" || name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\`) {
		return "", errors.New("invalid local source image")
	}
	mime := ""
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		mime = "image/png"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".webp":
		mime = "image/webp"
	default:
		return "", errors.New("source image must be PNG, JPEG or WebP")
	}
	path := filepath.Join(p.MediaDir, name)
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read source image: %w", err)
	}
	if info.Size() > 10<<20 || info.Size() == 0 {
		return "", errors.New("source image must be between 1 byte and 10 MB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read source image: %w", err)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
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
		TaskID:    item.ID,
		ProjectID: item.ProjectID,
		DraftID:   item.DraftID,
		Status:    item.Status,
		Progress:  item.Progress,
		Stage:     item.Stage,
		At:        item.UpdatedAt,
	})
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if !p.active(ctx, item) || ctx.Err() != nil {
			return
		}
		saved := true
		process := func(progress int, stage task.Stage) {
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
				TaskID:    item.ID,
				ProjectID: item.ProjectID,
				DraftID:   item.DraftID,
				Status:    item.Status,
				Progress:  item.Progress,
				Stage:     item.Stage,
				At:        item.UpdatedAt,
			})
		}
		if item.ModelID == "" {
			err = errors.New("legacy provider execution is unavailable; select a model")
		} else {
			err = p.processModel(ctx, &item, process)
		}
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
	if err == nil && p.Results != nil {
		err = p.Results.Apply(ctx, item)
		if err != nil {
			err = fmt.Errorf("apply task result: %w", err)
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
		TaskID:    item.ID,
		ProjectID: item.ProjectID,
		DraftID:   item.DraftID,
		Status:    item.Status,
		Progress:  item.Progress,
		Stage:     item.Stage,
		At:        item.UpdatedAt,
	})
}

func (p *Processor) active(ctx context.Context, item task.Task) bool {
	current, getErr := p.Task.Get(ctx, item.ID)
	return getErr == nil && current.Status == task.StatusRunning
}
