package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

func (r *TaskRepository) SyncModelIdentities(ctx context.Context, connection provider.Connection, entries []provider.DiscoveredModel, seen time.Time) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err = checkDeleted(ctx, tx, "connection", "", connection.ID); err != nil {
		return 0, err
	}
	var raw string
	if err = tx.QueryRowContext(ctx, "SELECT payload FROM provider_connections WHERE id=?", connection.ID).Scan(&raw); err != nil {
		return 0, err
	}
	var current provider.Connection
	if err = json.Unmarshal([]byte(raw), &current); err != nil {
		return 0, err
	}
	if current.Revision != connection.Revision {
		return 0, provider.ErrRevisionConflict
	}
	count := 0
	ids := map[string]bool{}
	for _, entry := range entries {
		if ids[entry.ID] {
			continue
		}
		ids[entry.ID] = true
		id := provider.ModelID(connection.ID, entry.ID)
		var m provider.Model
		err := tx.QueryRowContext(ctx, "SELECT payload FROM provider_models WHERE id=?", id).Scan(&raw)
		if err == nil {
			if err = json.Unmarshal([]byte(raw), &m); err != nil {
				return 0, err
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
		m, err = m.ApplyDiscoveredIdentity(connection.ID, entry, seen)
		if err != nil {
			return 0, err
		}
		payload, err := json.Marshal(m)
		if err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO provider_models(id,connection_id,payload) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", id, connection.ID, string(payload)); err != nil {
			return 0, err
		}
		count++
	}
	return count, tx.Commit()
}
