package application

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

func freezeTaskModel(ctx context.Context, models provider.ModelRepository, item *task.Task, connections ...provider.ConnectionRepository) error {
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
	return freezeResolvedTaskModel(ctx, m, item, connections...)
}

func freezeResolvedTaskModel(ctx context.Context, m provider.Model, item *task.Task, connections ...provider.ConnectionRepository) error {
	status := m.Resolve(provider.Capability(item.Kind))
	if !status.Available {
		return errors.New(status.Code + ": " + status.Reason)
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &item.ModelConfig); err != nil {
		return err
	}
	if len(connections) > 0 && connections[0] != nil {
		c, err := connections[0].GetConnection(ctx, m.ConnectionID)
		if err != nil {
			return err
		}
		c = c.GenerationSnapshot()
		item.ConnectionConfig = &c
	}
	return nil
}
