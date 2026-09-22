package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	_ "modernc.org/sqlite"
	"strings"
)

type TaskRepository struct{ db *sql.DB }

func Open(ctx context.Context, path string) (*TaskRepository, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.ExecContext(ctx, `PRAGMA busy_timeout = 5000; PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tasks (id TEXT PRIMARY KEY, project_id TEXT, draft_id TEXT, payload TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS projects (id TEXT PRIMARY KEY, payload TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS providers (id TEXT PRIMARY KEY, capability TEXT NOT NULL, payload TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS provider_connections (id TEXT PRIMARY KEY, payload TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS provider_models (id TEXT PRIMARY KEY, connection_id TEXT NOT NULL, payload TEXT NOT NULL);
		CREATE INDEX IF NOT EXISTS provider_models_connection ON provider_models(connection_id);
		CREATE TABLE IF NOT EXISTS materials (id TEXT PRIMARY KEY, kind TEXT NOT NULL, payload TEXT NOT NULL);
	`); err != nil {
		_ = db.Close()
		return nil, err
	}
	for _, column := range []struct{ name, definition string }{
		{"project_id", "TEXT"},
		{"draft_id", "TEXT"},
	} {
		if err = ensureColumn(ctx, db, "tasks", column.name, column.definition); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	if _, err = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS tasks_scope ON tasks(project_id, draft_id, id DESC)`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &TaskRepository{db: db}, nil
}

func ensureColumn(ctx context.Context, db *sql.DB, table, name, definition string) error {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid int
		var column, kind string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &column, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		if column == name {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = db.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+name+" "+definition)
	return err
}

func saveJSON(ctx context.Context, db *sql.DB, table, id string, value any, categoryColumn, category string) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if categoryColumn == "" {
		_, err = db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s(id,payload) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", table), id, string(b))
	} else {
		_, err = db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s(id,%s,payload) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET %s=excluded.%s,payload=excluded.payload", table, categoryColumn, categoryColumn, categoryColumn), id, category, string(b))
	}
	return err
}

func getJSON[T any](ctx context.Context, db *sql.DB, table, id string) (T, error) {
	var value T
	var raw string
	err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT payload FROM %s WHERE id=?", table), id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return value, fault.ErrNotFound
	}
	if err != nil {
		return value, err
	}
	err = json.Unmarshal([]byte(raw), &value)
	return value, err
}
func listJSON[T any](ctx context.Context, db *sql.DB, table, categoryColumn, category string) ([]T, error) {
	query := fmt.Sprintf("SELECT payload FROM %s ORDER BY id DESC", table)
	args := []any{}
	if categoryColumn != "" {
		query = fmt.Sprintf("SELECT payload FROM %s WHERE %s=? ORDER BY id DESC", table, categoryColumn)
		args = append(args, category)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var raw string
		var item T
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func deleteJSON(ctx context.Context, db *sql.DB, table, id string) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id=?", table), id)
	return err
}

func (r *TaskRepository) SaveProject(ctx context.Context, v project.Project) error {
	return saveJSON(ctx, r.db, "projects", v.ID, v, "", "")
}

func (r *TaskRepository) GetProject(ctx context.Context, id string) (project.Project, error) {
	return getJSON[project.Project](ctx, r.db, "projects", id)
}

func (r *TaskRepository) ListProjects(ctx context.Context) ([]project.Project, error) {
	return listJSON[project.Project](ctx, r.db, "projects", "", "")
}

func (r *TaskRepository) DeleteProject(ctx context.Context, id string) error {
	return deleteJSON(ctx, r.db, "projects", id)
}

func (r *TaskRepository) SaveProvider(ctx context.Context, v provider.Config) error {
	return saveJSON(ctx, r.db, "providers", v.Code, v, "capability", string(v.Capability))
}

func (r *TaskRepository) GetProvider(ctx context.Context, id string) (provider.Config, error) {
	return getJSON[provider.Config](ctx, r.db, "providers", id)
}

func (r *TaskRepository) ListProviders(ctx context.Context, kind provider.Capability) ([]provider.Config, error) {
	return listJSON[provider.Config](ctx, r.db, "providers", "capability", string(kind))
}

func (r *TaskRepository) DeleteProvider(ctx context.Context, id string) error {
	return deleteJSON(ctx, r.db, "providers", id)
}

func (r *TaskRepository) SaveConnection(ctx context.Context, v provider.Connection) error {
	return saveJSON(ctx, r.db, "provider_connections", v.ID, v, "", "")
}
func (r *TaskRepository) GetConnection(ctx context.Context, id string) (provider.Connection, error) {
	return getJSON[provider.Connection](ctx, r.db, "provider_connections", id)
}
func (r *TaskRepository) ListConnections(ctx context.Context) ([]provider.Connection, error) {
	return listJSON[provider.Connection](ctx, r.db, "provider_connections", "", "")
}
func (r *TaskRepository) DeleteConnection(ctx context.Context, id string) error {
	models, err := r.ListModels(ctx, id)
	if err != nil {
		return err
	}
	if len(models) != 0 {
		return errors.New("remove connection models before deleting connection")
	}
	return deleteJSON(ctx, r.db, "provider_connections", id)
}
func (r *TaskRepository) SaveModel(ctx context.Context, v provider.Model) error {
	return saveJSON(ctx, r.db, "provider_models", v.ID, v, "connection_id", v.ConnectionID)
}
func (r *TaskRepository) GetModel(ctx context.Context, id string) (provider.Model, error) {
	return getJSON[provider.Model](ctx, r.db, "provider_models", id)
}
func (r *TaskRepository) ListModels(ctx context.Context, connectionID string) ([]provider.Model, error) {
	if connectionID == "" {
		return listJSON[provider.Model](ctx, r.db, "provider_models", "", "")
	}
	return listJSON[provider.Model](ctx, r.db, "provider_models", "connection_id", connectionID)
}
func (r *TaskRepository) DeleteModel(ctx context.Context, id string) error {
	return deleteJSON(ctx, r.db, "provider_models", id)
}

func (r *TaskRepository) SaveMaterial(ctx context.Context, v material.Asset) error {
	return saveJSON(ctx, r.db, "materials", v.ID, v, "kind", string(v.Kind))
}

func (r *TaskRepository) GetMaterial(ctx context.Context, id string) (material.Asset, error) {
	return getJSON[material.Asset](ctx, r.db, "materials", id)
}

func (r *TaskRepository) ListMaterials(ctx context.Context, kind material.Kind) ([]material.Asset, error) {
	return listJSON[material.Asset](ctx, r.db, "materials", "kind", string(kind))
}

func (r *TaskRepository) DeleteMaterial(ctx context.Context, id string) error {
	return deleteJSON(ctx, r.db, "materials", id)
}

type ProjectRepository struct{ root *TaskRepository }

func NewProjectRepository(root *TaskRepository) *ProjectRepository {
	return &ProjectRepository{root: root}
}

func (r *ProjectRepository) Save(ctx context.Context, v project.Project) error {
	return r.root.SaveProject(ctx, v)
}

func (r *ProjectRepository) Get(ctx context.Context, id string) (project.Project, error) {
	return r.root.GetProject(ctx, id)
}

func (r *ProjectRepository) List(ctx context.Context) ([]project.Project, error) {
	return r.root.ListProjects(ctx)
}

func (r *ProjectRepository) Delete(ctx context.Context, id string) error {
	return r.root.DeleteProject(ctx, id)
}

func (r *ProjectRepository) Update(ctx context.Context, id string, change func(*project.Project) error) (project.Project, error) {
	var value project.Project
	tx, err := r.root.db.BeginTx(ctx, nil)
	if err != nil {
		return value, err
	}
	defer tx.Rollback()
	var raw string
	if err = tx.QueryRowContext(ctx, `SELECT payload FROM projects WHERE id=?`, id).Scan(&raw); errors.Is(err, sql.ErrNoRows) {
		return value, fault.ErrNotFound
	} else if err != nil {
		return value, err
	}
	if err = json.Unmarshal([]byte(raw), &value); err != nil {
		return value, err
	}
	if err = change(&value); err != nil {
		return value, err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return value, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE projects SET payload=? WHERE id=?`, string(payload), id); err != nil {
		return value, err
	}
	return value, tx.Commit()
}

type ProviderRepository struct{ root *TaskRepository }

func NewProviderRepository(root *TaskRepository) *ProviderRepository {
	return &ProviderRepository{root: root}
}

func (r *ProviderRepository) Save(ctx context.Context, v provider.Config) error {
	return r.root.SaveProvider(ctx, v)
}

func (r *ProviderRepository) Get(ctx context.Context, id string) (provider.Config, error) {
	return r.root.GetProvider(ctx, id)
}

func (r *ProviderRepository) List(ctx context.Context, k provider.Capability) ([]provider.Config, error) {
	return r.root.ListProviders(ctx, k)
}

func (r *ProviderRepository) Delete(ctx context.Context, id string) error {
	return r.root.DeleteProvider(ctx, id)
}

type MaterialRepository struct{ root *TaskRepository }

func NewMaterialRepository(root *TaskRepository) *MaterialRepository {
	return &MaterialRepository{root: root}
}

func (r *MaterialRepository) Save(ctx context.Context, v material.Asset) error {
	return r.root.SaveMaterial(ctx, v)
}

func (r *MaterialRepository) Get(ctx context.Context, id string) (material.Asset, error) {
	return r.root.GetMaterial(ctx, id)
}

func (r *MaterialRepository) List(ctx context.Context, k material.Kind) ([]material.Asset, error) {
	return r.root.ListMaterials(ctx, k)
}

func (r *MaterialRepository) Delete(ctx context.Context, id string) error {
	return r.root.DeleteMaterial(ctx, id)
}

func (r *TaskRepository) Close() error {
	return r.db.Close()
}

func (r *TaskRepository) Save(ctx context.Context, t task.Task) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO tasks(id,project_id,draft_id,payload) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET project_id=excluded.project_id,draft_id=excluded.draft_id,payload=excluded.payload`, t.ID, nullString(t.ProjectID), nullString(t.DraftID), string(b))
	return err
}
func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r *TaskRepository) Get(ctx context.Context, id string) (task.Task, error) {
	return getJSON[task.Task](ctx, r.db, "tasks", id)
}
func (r *TaskRepository) List(ctx context.Context) ([]task.Task, error) {
	return listJSON[task.Task](ctx, r.db, "tasks", "", "")
}

func (r *TaskRepository) ListByScope(ctx context.Context, scope task.Scope) ([]task.Task, error) {
	query := `SELECT payload FROM tasks`
	args := []any{}
	conditions := []string{}
	if scope.ProjectID != "" {
		conditions = append(conditions, "project_id=?")
		args = append(args, scope.ProjectID)
	}
	if scope.DraftID != "" {
		conditions = append(conditions, "draft_id=?")
		args = append(args, scope.DraftID)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY id DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []task.Task{}
	for rows.Next() {
		var raw string
		var item task.Task
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id=?`, id)
	return err
}
