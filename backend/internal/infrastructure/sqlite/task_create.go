package sqlite

import (
	"context"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

func (r *TaskRepository) CreateIfAbsent(ctx context.Context, t task.Task) (task.Task, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return t, false, err
	}
	defer tx.Rollback()
	b, err := json.Marshal(t)
	if err != nil {
		return t, false, err
	}
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO tasks(id,project_id,draft_id,payload) VALUES(?,?,?,?)`, t.ID, t.ProjectID, t.DraftID, string(b))
	if err != nil {
		return t, false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return t, false, err
	}
	if count == 0 {
		var raw string
		if err = tx.QueryRowContext(ctx, `SELECT payload FROM tasks WHERE id=?`, t.ID).Scan(&raw); err != nil {
			return t, false, err
		}
		err = json.Unmarshal([]byte(raw), &t)
		if err != nil {
			return t, false, err
		}
	} else if err = syncTaskReferences(ctx, tx, t); err != nil {
		return t, false, err
	}
	return t, count == 1, tx.Commit()
}
