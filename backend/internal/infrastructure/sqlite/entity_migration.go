package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

func syncProjectReferences(ctx context.Context, tx *sql.Tx, p project.Project) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM material_references WHERE project_id=? AND source_kind!='generation_snapshot'`, p.ID); err != nil {
		return err
	}
	add := func(d, s, m, k, id string) error {
		_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO material_references(project_id,draft_id,scene_id,material_id,source_kind,source_id) SELECT ?,?,?,?,?,? WHERE EXISTS(SELECT 1 FROM materials WHERE id=?)`, p.ID, d, s, m, k, id, m)
		return err
	}
	for _, e := range p.Entities {
		for _, r := range e.References {
			if err := add("", "", r.MaterialID, "entity", fmt.Sprintf("%s:%d", e.ID, e.Revision)); err != nil {
				return err
			}
		}
	}
	for _, d := range p.Drafts {
		for _, b := range d.Bindings {
			if err := add(d.ID, b.SceneID, b.MaterialID, "binding", b.Usage); err != nil {
				return err
			}
		}
		for _, snap := range d.StoryboardSnapshots {
			for _, take := range snap.MediaTakes {
				if err := add(d.ID, take.SceneID, take.MaterialID, "storyboard_snapshot", snap.ID); err != nil {
					return err
				}
			}
			for _, b := range snap.Bindings {
				if err := add(d.ID, b.SceneID, b.MaterialID, "storyboard_snapshot", snap.ID); err != nil {
					return err
				}
			}
		}
		for _, take := range d.MediaTakes {
			if err := add(d.ID, take.SceneID, take.MaterialID, "media_take", take.ID); err != nil {
				return err
			}
		}
		for _, c := range d.Compositions {
			if c.MusicMaterialID != "" {
				if err := add(d.ID, "", c.MusicMaterialID, "composition", c.ID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func syncTaskReferences(ctx context.Context, tx *sql.Tx, t task.Task) error {
	if t.Snapshot == nil {
		return nil
	}
	for _, e := range t.Snapshot.Entities {
		for _, r := range e.References {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO material_references(project_id,draft_id,scene_id,material_id,source_kind,source_id) VALUES(?,?,?,?,?,?)`, t.ProjectID, t.DraftID, t.SceneID, r.MaterialID, "generation_snapshot", t.Snapshot.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
func migrateEntities(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=2`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return tx.Commit()
	}
	rows, err := tx.QueryContext(ctx, `SELECT payload FROM projects`)
	if err != nil {
		return err
	}
	var values []project.Project
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		var p project.Project
		if err = json.Unmarshal([]byte(raw), &p); err != nil {
			rows.Close()
			return err
		}
		values = append(values, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, p := range values {
		if err = syncProjectReferences(ctx, tx, p); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES(2)`); err != nil {
		return err
	}
	return tx.Commit()
}
