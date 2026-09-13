package queue

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"testing"
	"time"
)

func TestWorkerRetriesAndFails(t *testing.T) {
	store := NewStore()
	attempts := 0
	worker := NewWorkerWithProcessor(store, ProcessorFunc(func(context.Context, task.Task, func(int, string)) error { attempts++; return errors.New("boom") }))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx)
	v := task.New("1", "video", time.Now())
	_ = store.Save(v)
	worker.Enqueue(v)
	time.Sleep(300 * time.Millisecond)
	got, _ := store.Get("1")
	if got.Status != task.StatusFailed || attempts != 3 || got.RetryCount != 3 {
		t.Fatal(got, attempts)
	}
}
