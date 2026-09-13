package application

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"time"
)

type TaskService struct {
	Store  queue.Store
	Worker *queue.Worker
}

func (s TaskService) Create(kind string) (task.Task, error) {
	return s.CreateWithProvider(kind, "")
}
func (s TaskService) CreateWithProvider(kind, providerCode string) (task.Task, error) {
	if kind != "story" && kind != "image" && kind != "video" {
		return task.Task{}, errors.New("unsupported task kind")
	}
	now := time.Now().UTC()
	t := task.New(now.Format("20060102150405.000000000"), kind, now)
	t.ProviderCode = providerCode
	if err := s.Store.Save(t); err != nil {
		return task.Task{}, err
	}
	s.event(t)
	s.Worker.Enqueue(t)
	return t, nil
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
	s.Worker.Enqueue(t)
	return t, nil
}
func (s TaskService) Get(id string) (task.Task, bool) { return s.Store.Get(id) }
func (s TaskService) List() []task.Task               { return s.Store.List() }
func (s TaskService) event(t task.Task) {
	if events, ok := s.Store.(task.EventRepository); ok {
		_ = events.AppendEvent(task.Event{TaskID: t.ID, Status: t.Status, Progress: t.Progress, Stage: t.Stage, At: t.UpdatedAt})
	}
}
