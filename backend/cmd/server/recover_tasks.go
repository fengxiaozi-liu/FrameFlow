package main

import (
	"context"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
)

func recoverModelTasks(ctx context.Context, store *sqlite.TaskRepository, enqueue func(context.Context, task.Task) error) error {
	items, err := store.List(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ModelID == "" || item.Status != task.StatusQueued && item.Status != task.StatusRunning {
			continue
		}
		if item.Status == task.StatusRunning {
			item.Status = task.StatusQueued
			item.Stage = task.StageQueued
			item.UpdatedAt = time.Now().UTC()
			if err := store.Save(ctx, item); err != nil {
				return err
			}
		}
		if err := enqueue(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
