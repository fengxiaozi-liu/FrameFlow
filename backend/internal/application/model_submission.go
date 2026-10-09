package application

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

func freezeTaskModel(ctx context.Context, models provider.ModelRepository, item *task.Task) error {
	if item.Kind == task.KindComposition || item.ModelID == "" {
		return nil
	}
	if models == nil {
		return errors.New("model repository unavailable")
	}
	m, err := models.GetModel(ctx, item.ModelID)
	if err != nil {
		return err
	}
	status := m.Resolve(provider.Capability(item.Kind))
	if !status.Available {
		return errors.New(status.Code + ": " + status.Reason)
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &item.ModelConfig)
}
