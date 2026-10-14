package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

func (r *ProjectRepository) DeleteFailedGenerationRun(ctx context.Context, projectID, draftID, sceneID, runID string, now time.Time) error {
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
	var run *task.GenerationRun
	for _, d := range p.Drafts {
		if d.ID != draftID {
			continue
		}
		for _, item := range d.GenerationRuns {
			if item.ID == runID && item.SceneID == sceneID {
				copy := item
				run = &copy
				break
			}
		}
	}
	if run == nil {
		return fault.ErrNotFound
	}
	ids := map[string]bool{}
	sources := map[string]bool{}
	for _, id := range run.TaskIDs {
		ids[id] = true
		sources[id] = true
	}
	for i, snapshot := range run.Plan {
		id := fmt.Sprintf("%s-%d", runID, i)
		ids[id] = true
		sources[id] = true
		if snapshot.ID != "" {
			sources[snapshot.ID] = true
		}
	}
	args := []any{runID}
	placeholders := []string{}
	for id := range ids {
		args = append(args, id)
		placeholders = append(placeholders, "?")
	}
	query := `SELECT payload FROM tasks WHERE json_extract(payload,'$.run_id')=?`
	if len(placeholders) > 0 {
		query += " OR id IN (" + strings.Join(placeholders, ",") + ")"
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	var children []task.Task
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			rows.Close()
			return err
		}
		var child task.Task
		if err = json.Unmarshal([]byte(value), &child); err != nil {
			rows.Close()
			return err
		}
		if child.RunID != runID && !ids[child.ID] {
			continue
		}
		if child.ProjectID != projectID || child.DraftID != draftID || child.SceneID != sceneID || (child.RunID != "" && child.RunID != runID) {
			rows.Close()
			return deletionConflict("run_scope_conflict", "任务归属不一致，无法删除")
		}
		children = append(children, child)
		ids[child.ID] = true
		sources[child.ID] = true
		if child.Snapshot != nil {
			sources[child.Snapshot.ID] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if err = p.RemoveFailedRun(draftID, sceneID, runID, children, now); err != nil {
		return err
	}
	for id := range ids {
		if _, err = tx.ExecContext(ctx, `DELETE FROM tasks WHERE id=? AND project_id=? AND draft_id=?`, id, projectID, draftID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO deleted_resources(kind,owner_id,id) VALUES('task','',?)`, id); err != nil {
			return err
		}
	}
	// Snapshot IDs can be shared by other runs; release only references without retained tasks.
	for source := range sources {
		if _, err = tx.ExecContext(ctx, `DELETE FROM material_references WHERE project_id=? AND draft_id=? AND source_kind='generation_snapshot' AND source_id=? AND NOT EXISTS(SELECT 1 FROM tasks WHERE project_id=? AND draft_id=? AND (id=? OR json_extract(payload,'$.snapshot.id')=?))`, projectID, draftID, source, projectID, draftID, source, source); err != nil {
			return err
		}
	}
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
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO deleted_resources(kind,owner_id,id) VALUES('generation_run',?,?)`, projectID, runID); err != nil {
		return err
	}
	return tx.Commit()
}
