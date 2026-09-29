package processor

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	"path/filepath"
	"testing"
	"time"
)

func TestMediaDownloadRetryDoesNotRegenerate(t *testing.T) {
	item := task.New("download", task.KindSpeech, task.Input{}, time.Now())
	item.ResultURL = "https://example.com/result.wav"
	p := New(nil, nil)
	calls := 0
	p.SaveResult = func(context.Context, string, task.Kind, string) (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("temporary download failure")
		}
		return "/media/result.wav", nil
	}
	if p.processModel(context.Background(), &item, func(int, task.Stage) {}) == nil {
		t.Fatal("expected first download failure")
	}
	if err := p.processModel(context.Background(), &item, func(int, task.Stage) {}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || item.ResultURL != "/media/result.wav" {
		t.Fatal(item, calls)
	}
}

func TestLocalCompositionDoesNotRequireRemoteModel(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	item := task.New("composition", task.KindComposition, task.Input{Prompt: "compose"}, time.Now())
	item.Composition = &task.CompositionInput{}
	if err = db.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	p := New(nil, db)
	calls := 0
	p.Compose = func(context.Context, string, task.CompositionInput, func(int, task.Stage)) (string, float64, error) {
		calls++
		return "/media/output.mp4", 6, nil
	}
	p.Execute(ctx, item)
	got, _ := db.Get(ctx, item.ID)
	if got.Status != task.StatusSucceeded || calls != 1 {
		t.Fatal(got, calls)
	}
}
