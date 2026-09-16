package queue

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"testing"
	"time"
)

func TestEnqueueCanCancelWhileQueueIsFull(t *testing.T) {
	worker := NewWorker()
	for i := 0; i < cap(worker.jobs); i++ {
		if err := worker.Enqueue(context.Background(), task.Task{}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := worker.Enqueue(ctx, task.Task{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if worker.Depth() != cap(worker.jobs) {
		t.Fatal("cancelled enqueue changed queue")
	}
}

func TestWorkerProcessesSeriallyAndStops(t *testing.T) {
	worker := NewWorker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan string, 2)
	done := make(chan struct{})
	worker.Enqueue(context.Background(), task.Task{ID: "first"})
	worker.Enqueue(context.Background(), task.Task{ID: "second"})
	go func() {
		defer close(done)
		worker.Run(ctx, func(_ context.Context, item task.Task) {
			if item.ID == "first" {
				close(first)
				<-release
			}
			completed <- item.ID
		})
	}()
	<-first
	if worker.Depth() != 1 {
		t.Error("worker consumed next task before processor returned")
	}
	close(release)
	for _, want := range []string{"first", "second"} {
		select {
		case got := <-completed:
			if got != want {
				t.Fatalf("got %s, want %s", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("processor was not called")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}
