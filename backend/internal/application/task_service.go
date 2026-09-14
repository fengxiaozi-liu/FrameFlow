package application

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"time"
)

type TaskService struct {
	Store     queue.Store
	Enqueuer  interface{ Enqueue(task.Task) }
	Providers ProviderService
}

func (s TaskService) Create(kind string, input task.Input) (task.Task, error) {
	return s.CreateWithProvider(kind, "", input)
}

func (s TaskService) CreateWithProvider(kind, providerCode string, input task.Input) (task.Task, error) {
	if err := input.Validate(); err != nil {
		return task.Task{}, err
	}
	kindValue, err := task.ParseKind(kind)
	if err != nil {
		return task.Task{}, err
	}
	now := time.Now().UTC()
	t := task.New(now.Format("20060102150405.000000000"), kindValue, input, now)
	t.ProviderCode = providerCode
	if err := s.Store.Save(t); err != nil {
		return task.Task{}, err
	}
	s.event(t)
	if s.Enqueuer != nil {
		s.Enqueuer.Enqueue(t)
	}
	return t, nil
}

func (s TaskService) CreateGeneration(kind, providerCode string, input task.Input) (task.Task, error) {
	kindValue, err := task.ParseKind(kind)
	if err != nil {
		return task.Task{}, err
	}
	expected := map[task.Kind]provider.Capability{
		task.KindStory: provider.Story,
		task.KindImage: provider.Image,
		task.KindVideo: provider.Video,
	}[kindValue]
	if expected == "" {
		return task.Task{}, errors.New("unsupported generation kind")
	}
	config, ok := s.Providers.Get(providerCode)
	if !ok {
		return task.Task{}, errors.New("provider not found")
	}
	if config.Capability != expected {
		return task.Task{}, errors.New("provider capability mismatch")
	}
	if !config.Enabled || config.Status != provider.Healthy {
		return task.Task{}, errors.New("provider is not ready")
	}
	return s.CreateWithProvider(kind, providerCode, input)
}

func (s TaskService) CreateCharacterImage(providerCode string, input task.Input) (task.Task, error) {
	return s.CreateGeneration(string(task.KindImage), providerCode, input)
}

func (s TaskService) CreateFrameImage(providerCode string, input task.Input) (task.Task, error) {
	return s.CreateGeneration(string(task.KindImage), providerCode, input)
}

func (s TaskService) Cancel(id string) (task.Task, error) {
	t, ok := s.Store.Get(id)
	if !ok {
		return task.Task{}, errors.New("task not found")
	}
	if err := t.Cancel(time.Now().UTC()); err != nil {
		return task.Task{}, err
	}
	if err := s.Store.Save(t); err != nil {
		return t, err
	}
	s.event(t)
	return t, nil
}

func (s TaskService) Retry(id string) (task.Task, error) {
	t, ok := s.Store.Get(id)
	if !ok {
		return task.Task{}, errors.New("task not found")
	}
	if err := t.Retry(time.Now().UTC()); err != nil {
		return task.Task{}, err
	}
	if err := s.Store.Save(t); err != nil {
		return task.Task{}, err
	}
	s.event(t)
	if s.Enqueuer != nil {
		s.Enqueuer.Enqueue(t)
	}
	return t, nil
}

func (s TaskService) Get(id string) (task.Task, bool) {
	return s.Store.Get(id)
}

func (s TaskService) List() []task.Task {
	return s.Store.List()
}

func (s TaskService) event(t task.Task) {
	if events, ok := s.Store.(task.EventRepository); ok {
		_ = events.AppendEvent(task.Event{TaskID: t.ID, Status: t.Status, Progress: t.Progress, Stage: t.Stage, At: t.UpdatedAt})
	}
}
