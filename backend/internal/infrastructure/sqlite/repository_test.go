package sqlite

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"path/filepath"
	"testing"
	"time"
)

func TestRepositoriesPersistAggregates(t *testing.T) {
	r, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	now := time.Now().UTC()
	taskValue := task.New("t1", task.KindVideo, task.Input{Prompt: "测试视频"}, now)
	if err = r.Save(context.Background(), taskValue); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get(context.Background(), "t1"); ok != nil {
		t.Fatal("task missing")
	}
	p, _ := project.New("p1", "Demo", now)
	if err = r.SaveProject(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.GetProject(context.Background(), "p1"); ok != nil {
		t.Fatal("project missing")
	}
	c, _ := provider.New("v1", "Video", provider.Video)
	if err = r.SaveProvider(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if items, err := r.ListProviders(context.Background(), provider.Video); err != nil || len(items) != 1 {
		t.Fatal("provider missing")
	}
	a, _ := material.New("m1", "Cover", material.Visual, now)
	if err = r.SaveMaterial(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if items, err := r.ListMaterials(context.Background(), material.Visual); err != nil || len(items) != 1 {
		t.Fatal("material missing")
	}
}

func TestBackupCanBeRestoredAndPassesIntegrityCheck(t *testing.T) {
	dir := t.TempDir()
	original, err := Open(context.Background(), filepath.Join(dir, "original.db"))
	if err != nil {
		t.Fatal(err)
	}
	value := task.New("backup-task", task.KindVideo, task.Input{Prompt: "备份视频"}, time.Now().UTC())
	if err = original.Save(context.Background(), value); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(dir, "backup", "frameflow.db")
	if err = original.Backup(context.Background(), backupPath); err != nil {
		t.Fatal(err)
	}
	_ = original.Close()

	restored, err := Open(context.Background(), backupPath)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if err = restored.VerifyIntegrity(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := restored.Get(context.Background(), value.ID); ok != nil {
		t.Fatal("restored database is missing the saved task")
	}
}
