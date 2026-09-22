package application

import (
	"context"
	"errors"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type TaskResultApplier struct {
	Projects project.Repository
}

func (a TaskResultApplier) Apply(ctx context.Context, item task.Task) error {
	if a.Projects == nil {
		return errors.New("project repository is required to apply task result")
	}
	if err := item.ValidateScope(); err != nil {
		return err
	}
	output := project.DraftOutput{
		TaskID: item.ID,
		Kind:   string(item.Kind),
		Text:   item.ResultText,
		URL:    item.ResultURL,
	}
	now := time.Now().UTC()
	change := func(value *project.Project) error {
		return value.ApplyTaskResult(item.DraftID, output, now)
	}
	if atomic, ok := a.Projects.(project.AtomicRepository); ok {
		_, err := atomic.Update(ctx, item.ProjectID, change)
		return err
	}
	value, err := a.Projects.Get(ctx, item.ProjectID)
	if err != nil {
		return err
	}
	if err = change(&value); err != nil {
		return err
	}
	return a.Projects.Save(ctx, value)
}
