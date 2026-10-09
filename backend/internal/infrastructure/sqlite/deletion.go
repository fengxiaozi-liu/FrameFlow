package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

func deletionConflict(code, message string) error {
	return &fault.Error{Kind: "conflict", Code: code, Cause: errors.New(message)}
}

func checkDeleted(ctx context.Context, tx *sql.Tx, kind, owner, id string) error {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM deleted_resources WHERE kind=? AND owner_id=? AND id=?`, kind, owner, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return deletionConflict("resource_deleted", "该资源已删除，请刷新页面后重新选择")
	}
	return nil
}
func checkDeletedDrafts(ctx context.Context, tx *sql.Tx, p project.Project) error {
	for _, d := range p.Drafts {
		if err := checkDeleted(ctx, tx, "draft", p.ID, d.ID); err != nil {
			return err
		}
	}
	return nil
}
func checkTaskDeletion(ctx context.Context, tx *sql.Tx, t task.Task) error {
	if t.Status != task.StatusQueued && t.Status != task.StatusRunning {
		return nil
	}
	if err := checkDeleted(ctx, tx, "draft", t.ProjectID, t.DraftID); err != nil {
		return err
	}
	return checkDeleted(ctx, tx, "model", "", t.ModelID)
}
func (r *ProjectRepository) DeleteDraft(ctx context.Context, projectID, draftID string, expectedVersion int64) error {
	tx, err := r.root.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw string
	if err = tx.QueryRowContext(ctx, `SELECT payload FROM projects WHERE id=?`, projectID).Scan(&raw); errors.Is(err, sql.ErrNoRows) {
		return fault.ErrNotFound
	} else if err != nil {
		return err
	}
	var p project.Project
	if err = json.Unmarshal([]byte(raw), &p); err != nil {
		return err
	}
	index := -1
	for i, d := range p.Drafts {
		if d.ID == draftID {
			index = i
			break
		}
	}
	if index < 0 {
		return fault.ErrNotFound
	}
	if p.Drafts[index].Version != expectedVersion {
		return deletionConflict("draft_version_conflict", "草稿已在其他页面更新，请刷新后确认再删除")
	}
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE project_id=? AND draft_id=? AND json_extract(payload,'$.status') IN ('queued','running')`, projectID, draftID).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return deletionConflict("draft_in_use", "该草稿有排队或进行中的任务，请完成或取消任务后再删除")
	}
	p.Drafts = append(p.Drafts[:index], p.Drafts[index+1:]...)
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE projects SET payload=? WHERE id=?`, string(payload), projectID); err != nil {
		return err
	}
	if err = syncProjectReferences(ctx, tx, p); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO deleted_resources(kind,owner_id,id) VALUES('draft',?,?)`, projectID, draftID); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *TaskRepository) deleteConnectionCascade(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE json_extract(payload,'$.status') IN ('queued','running') AND json_extract(payload,'$.model_id') IN (SELECT id FROM provider_models WHERE connection_id=?)`, id).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return deletionConflict("connection_in_use", "该厂商有排队或进行中的任务，请完成或取消任务后再删除")
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO deleted_resources(kind,owner_id,id) SELECT 'model','',id FROM provider_models WHERE connection_id=?`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM provider_models WHERE connection_id=?`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM provider_connections WHERE id=?`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO deleted_resources(kind,owner_id,id) VALUES('connection','',?)`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Reject late model syncs and connection edits after a confirmed deletion.
func (r *TaskRepository) saveCatalogRecord(ctx context.Context, id, connectionID string, value any, model bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	owner := connectionID
	if !model {
		owner = id
	}
	if err = checkDeleted(ctx, tx, "connection", "", owner); err != nil {
		return err
	}
	if model {
		var raw string
		if e := tx.QueryRowContext(ctx, "SELECT payload FROM provider_models WHERE id=?", id).Scan(&raw); e == nil {
			var old provider.Model
			if e = json.Unmarshal([]byte(raw), &old); e != nil {
				return e
			}
			old = provider.MigrateLegacyModel(old)
			next := value.(provider.Model)
			if next.Revision != old.Revision {
				return provider.ErrRevisionConflict
			}
		} else if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if !model {
		_, err = tx.ExecContext(ctx, `INSERT INTO provider_connections(id,payload) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload`, id, string(b))
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO provider_models(id,connection_id,payload) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload,connection_id=excluded.connection_id`, id, connectionID, string(b))
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
