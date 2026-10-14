package application

import (
	"context"
	"time"
)

type GenerationRunDeletionRepository interface {
	DeleteFailedGenerationRun(context.Context, string, string, string, string, time.Time) error
}

type GenerationRunDeletion struct {
	Repository GenerationRunDeletionRepository
}

func (s GenerationRunDeletion) Delete(ctx context.Context, projectID, draftID, sceneID, runID string) error {
	if s.Repository == nil {
		return unavailable("run_delete_unavailable", "任务删除暂不可用")
	}
	// Serialize with scheduling and retries; the repository checks persisted states in its transaction.
	generationRunTransition.Lock()
	defer generationRunTransition.Unlock()
	return s.Repository.DeleteFailedGenerationRun(ctx, projectID, draftID, sceneID, runID, time.Now().UTC())
}
