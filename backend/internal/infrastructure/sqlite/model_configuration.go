package sqlite

import (
	"context"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

func (r *TaskRepository) ConfigureOperations(ctx context.Context, id string, expected int64, bindings map[provider.Capability]provider.OperationBinding, capabilities []provider.Capability) (provider.Model, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return provider.Model{}, err
	}
	defer tx.Rollback()
	var raw string
	if err = tx.QueryRowContext(ctx, "SELECT payload FROM provider_models WHERE id=?", id).Scan(&raw); err != nil {
		return provider.Model{}, err
	}
	var m provider.Model
	if err = json.Unmarshal([]byte(raw), &m); err != nil {
		return m, err
	}
	m = provider.MigrateLegacyModel(m)
	if m.Revision != expected {
		return m, provider.ErrRevisionConflict
	}
	if err = checkDeleted(ctx, tx, "connection", "", m.ConnectionID); err != nil {
		return m, err
	}
	m, err = m.Configure(expected, bindings, capabilities)
	if err != nil {
		return m, err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return m, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE provider_models SET payload=? WHERE id=?", string(b), id)
	if err != nil {
		return m, err
	}
	return m, tx.Commit()
}
