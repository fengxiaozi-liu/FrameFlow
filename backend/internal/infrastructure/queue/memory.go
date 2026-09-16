package queue

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"sync"
)

type Store interface {
	Save(context.Context, task.Task) error
	Get(context.Context, string) (task.Task, error)
	List(context.Context) ([]task.Task, error)
	Delete(context.Context, string) error
}
type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]task.Task
}

func NewStore() *MemoryStore {
	return &MemoryStore{items: map[string]task.Task{}}
}

func (s *MemoryStore) Save(ctx context.Context, t task.Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[t.ID] = t
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, id string) (task.Task, error) {
	if err := ctx.Err(); err != nil {
		return task.Task{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.items[id]
	if !ok {
		return t, fault.ErrNotFound
	}
	return t, nil
}

func (s *MemoryStore) List(ctx context.Context) ([]task.Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]task.Task, 0, len(s.items))
	for _, t := range s.items {
		out = append(out, t)
	}
	return out, nil
}

func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return nil
}

// Worker executes queued tasks serially in the goroutine running Run.
type Worker struct{ jobs chan task.Task }

func NewWorker() *Worker { return &Worker{jobs: make(chan task.Task, 32)} }
func (w *Worker) Enqueue(ctx context.Context, t task.Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case w.jobs <- t:
		return nil
	}
}
func (w *Worker) Depth() int { return len(w.jobs) }
func (w *Worker) Run(ctx context.Context, process func(context.Context, task.Task)) {
	for {
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case item := <-w.jobs:
			process(ctx, item)
		}
	}
}
