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
		if (item.ModelID == "" && item.Kind != task.KindComposition) || item.Status != task.StatusQueued && item.Status != task.StatusRunning {
			continue
		}
		if item.Status == task.StatusRunning {
			if item.Snapshot != nil && item.RemoteTaskID == "" && item.ResultURL == "" {
				item.Fail("进程中断，服务端是否完成未知；请检查后手动重试，避免重复计费", time.Now().UTC())
				if err := store.Save(ctx, item); err != nil {
					return err
				}
				continue
			}
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
