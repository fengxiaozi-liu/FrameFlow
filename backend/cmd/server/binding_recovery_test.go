package main

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotRecoveryPreservesUncertainAndCompletedRequests(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "recover.db")
	db, err := sqlite.Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"queued", "uncertain", "remote", "done", "composition"} {
		v := task.New(id, task.KindVideo, task.Input{Prompt: "test"}, time.Now())
		v.ModelID = "m"
		v.Snapshot = &task.GenerationSnapshot{ID: "s"}
		if id == "composition" {
			v.Kind, v.ModelID, v.Snapshot = task.KindComposition, "", nil
		}
		if id != "queued" {
			v.Status = task.StatusRunning
		}
		if id == "remote" {
			v.RemoteTaskID = "vendor-job"
		}
		if id == "done" {
			v.Status = task.StatusSucceeded
		}
		if err = db.Save(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	db, err = sqlite.Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	queued := map[string]bool{}
	if err = recoverModelTasks(ctx, db, func(_ context.Context, v task.Task) error { queued[v.ID] = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if len(queued) != 3 || !queued["queued"] || !queued["remote"] || !queued["composition"] {
		t.Fatal(queued)
	}
	v, _ := db.Get(ctx, "uncertain")
	if v.Status != task.StatusFailed {
		t.Fatal(v)
	}
	v, _ = db.Get(ctx, "done")
	if v.Status != task.StatusSucceeded {
		t.Fatal(v)
	}
}
