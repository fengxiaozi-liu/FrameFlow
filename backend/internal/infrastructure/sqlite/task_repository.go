package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	_ "modernc.org/sqlite"
)

type TaskRepository struct{ db *sql.DB }

func Open(path string) (*TaskRepository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA busy_timeout = 5000; PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (id TEXT PRIMARY KEY, payload TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS projects (id TEXT PRIMARY KEY, payload TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS providers (id TEXT PRIMARY KEY, capability TEXT NOT NULL, payload TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS materials (id TEXT PRIMARY KEY, kind TEXT NOT NULL, payload TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS task_events (sequence INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL, payload TEXT NOT NULL);
		CREATE INDEX IF NOT EXISTS idx_task_events_task ON task_events(task_id,sequence);
	`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &TaskRepository{db: db}, nil
}

func saveJSON(db *sql.DB, table, id string, value any, categoryColumn, category string) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if categoryColumn == "" {
		_, err = db.Exec(fmt.Sprintf("INSERT INTO %s(id,payload) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", table), id, string(b))
	} else {
		_, err = db.Exec(fmt.Sprintf("INSERT INTO %s(id,%s,payload) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET %s=excluded.%s,payload=excluded.payload", table, categoryColumn, categoryColumn, categoryColumn), id, category, string(b))
	}
	return err
}

func getJSON[T any](db *sql.DB, table, id string) (T, bool) {
	var zero T
	var raw string
	if db.QueryRow(fmt.Sprintf("SELECT payload FROM %s WHERE id=?", table), id).Scan(&raw) != nil {
		return zero, false
	}
	if json.Unmarshal([]byte(raw), &zero) != nil {
		var empty T
		return empty, false
	}
	return zero, true
}

func listJSON[T any](db *sql.DB, table, categoryColumn, category string) []T {
	query := fmt.Sprintf("SELECT payload FROM %s ORDER BY id DESC", table)
	args := []any{}
	if categoryColumn != "" {
		query = fmt.Sprintf("SELECT payload FROM %s WHERE %s=? ORDER BY id DESC", table, categoryColumn)
		args = append(args, category)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var raw string
		var item T
		if rows.Scan(&raw) == nil && json.Unmarshal([]byte(raw), &item) == nil {
			out = append(out, item)
		}
	}
	return out
}

func deleteJSON(db *sql.DB, table, id string) error {
	_, err := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE id=?", table), id)
	return err
}

func (r *TaskRepository) SaveProject(v project.Project) error {
	return saveJSON(r.db, "projects", v.ID, v, "", "")
}

func (r *TaskRepository) GetProject(id string) (project.Project, bool) {
	return getJSON[project.Project](r.db, "projects", id)
}

func (r *TaskRepository) ListProjects() []project.Project {
	return listJSON[project.Project](r.db, "projects", "", "")
}

func (r *TaskRepository) DeleteProject(id string) error {
	return deleteJSON(r.db, "projects", id)
}

func (r *TaskRepository) SaveProvider(v provider.Config) error {
	return saveJSON(r.db, "providers", v.Code, v, "capability", string(v.Capability))
}

func (r *TaskRepository) GetProvider(id string) (provider.Config, bool) {
	return getJSON[provider.Config](r.db, "providers", id)
}

func (r *TaskRepository) ListProviders(kind provider.Capability) []provider.Config {
	return listJSON[provider.Config](r.db, "providers", "capability", string(kind))
}

func (r *TaskRepository) DeleteProvider(id string) error {
	return deleteJSON(r.db, "providers", id)
}

func (r *TaskRepository) SaveMaterial(v material.Asset) error {
	return saveJSON(r.db, "materials", v.ID, v, "kind", string(v.Kind))
}

func (r *TaskRepository) GetMaterial(id string) (material.Asset, bool) {
	return getJSON[material.Asset](r.db, "materials", id)
}

func (r *TaskRepository) ListMaterials(kind material.Kind) []material.Asset {
	return listJSON[material.Asset](r.db, "materials", "kind", string(kind))
}

func (r *TaskRepository) DeleteMaterial(id string) error {
	return deleteJSON(r.db, "materials", id)
}

type ProjectRepository struct{ root *TaskRepository }

func NewProjectRepository(root *TaskRepository) *ProjectRepository {
	return &ProjectRepository{root: root}
}

func (r *ProjectRepository) Save(v project.Project) error {
	return r.root.SaveProject(v)
}

func (r *ProjectRepository) Get(id string) (project.Project, bool) {
	return r.root.GetProject(id)
}

func (r *ProjectRepository) List() []project.Project {
	return r.root.ListProjects()
}

func (r *ProjectRepository) Delete(id string) error {
	return r.root.DeleteProject(id)
}

type ProviderRepository struct{ root *TaskRepository }

func NewProviderRepository(root *TaskRepository) *ProviderRepository {
	return &ProviderRepository{root: root}
}

func (r *ProviderRepository) Save(v provider.Config) error {
	return r.root.SaveProvider(v)
}

func (r *ProviderRepository) Get(id string) (provider.Config, bool) {
	return r.root.GetProvider(id)
}

func (r *ProviderRepository) List(k provider.Capability) []provider.Config {
	return r.root.ListProviders(k)
}

func (r *ProviderRepository) Delete(id string) error {
	return r.root.DeleteProvider(id)
}

type MaterialRepository struct{ root *TaskRepository }

func NewMaterialRepository(root *TaskRepository) *MaterialRepository {
	return &MaterialRepository{root: root}
}

func (r *MaterialRepository) Save(v material.Asset) error {
	return r.root.SaveMaterial(v)
}

func (r *MaterialRepository) Get(id string) (material.Asset, bool) {
	return r.root.GetMaterial(id)
}

func (r *MaterialRepository) List(k material.Kind) []material.Asset {
	return r.root.ListMaterials(k)
}

func (r *MaterialRepository) Delete(id string) error {
	return r.root.DeleteMaterial(id)
}

func (r *TaskRepository) Close() error {
	return r.db.Close()
}

func (r *TaskRepository) Save(t task.Task) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`INSERT INTO tasks(id,payload) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload`, t.ID, string(b))
	return err
}

func (r *TaskRepository) Get(id string) (task.Task, bool) {
	var raw string
	if r.db.QueryRow(`SELECT payload FROM tasks WHERE id=?`, id).Scan(&raw) != nil {
		return task.Task{}, false
	}
	var t task.Task
	if json.Unmarshal([]byte(raw), &t) != nil {
		return task.Task{}, false
	}
	return t, true
}

func (r *TaskRepository) List() []task.Task {
	rows, err := r.db.Query(`SELECT payload FROM tasks ORDER BY id DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []task.Task{}
	for rows.Next() {
		var raw string
		if rows.Scan(&raw) == nil {
			var t task.Task
			if json.Unmarshal([]byte(raw), &t) == nil {
				out = append(out, t)
			}
		}
	}
	return out
}

func (r *TaskRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM tasks WHERE id=?`, id)
	return err
}

func (r *TaskRepository) AppendEvent(v task.Event) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	_, e = r.db.Exec(`INSERT INTO task_events(task_id,payload) VALUES(?,?)`, v.TaskID, string(b))
	return e
}

func (r *TaskRepository) ListEvents(id string, after int64) []task.Event {
	rows, e := r.db.Query(`SELECT sequence,payload FROM task_events WHERE task_id=? AND sequence>? ORDER BY sequence`, id, after)
	if e != nil {
		return nil
	}
	defer rows.Close()
	o := []task.Event{}
	for rows.Next() {
		var seq int64
		var raw string
		if rows.Scan(&seq, &raw) == nil {
			var v task.Event
			if json.Unmarshal([]byte(raw), &v) == nil {
				v.Sequence = seq
				o = append(o, v)
			}
		}
	}
	return o
}
