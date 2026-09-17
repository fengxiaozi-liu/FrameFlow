package sqlite

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"path/filepath"
	"testing"
	"time"
)

func TestRepositoryPropagatesCancellationAndStorageErrors(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "context.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	item := task.New("cancelled", task.KindVideo, task.Input{Prompt: "test"}, time.Now())
	if err := store.Save(ctx, item); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, item.ID); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := store.List(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), item.ID); !errors.Is(err, fault.ErrNotFound) {
		t.Fatal("cancelled save wrote task", err)
	}
	// The only connection is occupied: the query must stop waiting on its deadline.
	conn, err := store.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	deadline, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	_, err = store.List(deadline)
	_ = conn.Close()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	_ = store.Close()
	if _, err := store.List(context.Background()); err == nil {
		t.Fatal("closed database was reported as an empty list")
	}
}
