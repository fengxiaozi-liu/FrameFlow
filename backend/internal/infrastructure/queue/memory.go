package queue

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"sync"
	"time"
)

type Store interface {
	Save(task.Task) error
	Get(string) (task.Task, bool)
	List() []task.Task
	Delete(string) error
}
type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]task.Task
}

func NewStore() *MemoryStore {
	return &MemoryStore{items: map[string]task.Task{}}
}

func (s *MemoryStore) Save(t task.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[t.ID] = t
	return nil
}

func (s *MemoryStore) Get(id string) (task.Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.items[id]
	return t, ok
}

func (s *MemoryStore) List() []task.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]task.Task, 0, len(s.items))
	for _, t := range s.items {
		out = append(out, t)
	}
	return out
}

func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	return nil
}

type Worker struct {
	store Store
	jobs  chan task.Task
}

func NewWorker(s Store) *Worker {
	return &Worker{store: s, jobs: make(chan task.Task, 32)}
}

func (w *Worker) Enqueue(t task.Task) {
	w.jobs <- t
}

func (w *Worker) Depth() int {
	return len(w.jobs)
}

func (w *Worker) Run(ctx context.Context, process func(context.Context, task.Task, func(int, string)) error) {
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-w.jobs:
			w.process(ctx, t, process)
		}
	}
}

func (w *Worker) process(ctx context.Context, t task.Task, process func(context.Context, task.Task, func(int, string)) error) {
	current, ok := w.store.Get(t.ID)
	if !ok || current.Status != task.StatusQueued {
		return
	}
	t = current
	t.Start(time.Now().UTC())
	t.Stage = "preparing"
	w.save(t)
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = process(ctx, t, func(p int, stage string) {
			current, ok := w.store.Get(t.ID)
			if !ok || current.Status == task.StatusCancelled {
				return
			}
			t.Advance(p, stage, time.Now().UTC())
			w.save(t)
		})
		if err == nil {
			break
		}
		t.RetryCount = attempt + 1
		if attempt < 2 {
			time.Sleep(time.Duration(1<<attempt) * 25 * time.Millisecond)
		}
	}
	if err != nil {
		t.Fail(err.Error(), time.Now().UTC())
		w.save(t)
		return
	}
	if current, ok := w.store.Get(t.ID); !ok || current.Status == task.StatusCancelled {
		return
	}
	t.Succeed(time.Now().UTC())
	w.save(t)
}

func (w *Worker) save(t task.Task) {
	_ = w.store.Save(t)
	if events, ok := w.store.(task.EventRepository); ok {
		_ = events.AppendEvent(task.Event{TaskID: t.ID, Status: t.Status, Progress: t.Progress, Stage: t.Stage, At: t.UpdatedAt})
	}
}
