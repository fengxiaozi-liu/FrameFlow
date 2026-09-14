package http

import (
	"encoding/json"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/http/routes"
	ws "github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/websocket"
	"github.com/gin-gonic/gin"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	Store       queue.Store
	Worker      *queue.Worker
	TaskService application.TaskService
	Projects    application.ProjectService
	Providers   application.ProviderService
	Materials   application.MaterialService
	AuthToken   string
	UploadDir   string
	Vault       provider.CredentialVault
	RateLimit   int
	StaticFS    fs.FS
	Shutdown    func()
}

func (s Server) Routes() http.Handler {
	engine := gin.New()
	var media, frontend http.Handler
	if s.UploadDir != "" {
		media = http.StripPrefix("/media/", http.FileServer(http.Dir(s.UploadDir)))
	} else {
		media = http.NotFoundHandler()
	}
	if s.StaticFS != nil {
		frontend = spa(s.StaticFS)
	} else {
		frontend = http.NotFoundHandler()
	}
	routes.Register(engine, routes.Dependencies{
		Health: http.HandlerFunc(health), Metrics: http.HandlerFunc(s.metrics), Shutdown: http.HandlerFunc(s.shutdown), Tasks: http.HandlerFunc(s.Tasks), Task: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/event-log") {
				if events, ok := s.Store.(task.EventRepository); ok {
					TaskEvents(events)(w, r)
					return
				}
				writeError(w, 501, "events_unavailable", errors.New("event storage unavailable"))
				return
			}
			if strings.HasSuffix(r.URL.Path, "/events") {
				if events, ok := s.Store.(interface {
					queue.Store
					task.EventRepository
				}); ok {
					ws.TaskEvents(events)(w, r)
					return
				}
				writeError(w, 501, "events_unavailable", errors.New("events storage unavailable"))
				return
			}
			s.Task(w, r)
		}), Projects: http.HandlerFunc(s.projects), Project: http.HandlerFunc(s.project), Overview: http.HandlerFunc(s.overview), Providers: http.HandlerFunc(s.providers), Provider: http.HandlerFunc(s.provider), Materials: http.HandlerFunc(s.materials), Material: http.HandlerFunc(s.material), Media: media, SPA: frontend,
	})
	return audit(cors(newLimiter(s.RateLimit).wrap(auth(engine, s.AuthToken))))
}

func (s Server) shutdown(w http.ResponseWriter, _ *http.Request) {
	if s.Shutdown == nil {
		writeError(w, http.StatusNotImplemented, "shutdown_unavailable", errors.New("shutdown is unavailable"))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "shutting_down"})
	go s.Shutdown()
}

func spa(files fs.FS) http.Handler {
	assets := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" {
			if _, err := fs.Stat(files, name); err == nil {
				assets.ServeHTTP(w, r)
				return
			}
		}
		content, err := fs.ReadFile(files, "index.html")
		if err != nil {
			http.Error(w, "frontend unavailable", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(content)
	})
}

func health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy", "service": "frameflow"})
}

func (s Server) Tasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPost {
		var input struct {
			Kind           string `json:"kind"`
			ProviderCode   string `json:"provider_code"`
			Prompt         string `json:"prompt"`
			AspectRatio    string `json:"aspect_ratio"`
			SourceImageURL string `json:"source_image_url"`
		}
		if err := decode(r, &input); err != nil {
			writeError(w, 400, "invalid_request", err)
			return
		}
		if input.Kind == "" {
			input.Kind = string(task.KindVideo)
		}
		var t task.Task
		var err error
		taskInput := task.Input{Prompt: input.Prompt, AspectRatio: input.AspectRatio, SourceImageURL: input.SourceImageURL}
		if input.ProviderCode != "" {
			t, err = s.TaskService.CreateGeneration(input.Kind, input.ProviderCode, taskInput)
		} else {
			t, err = s.TaskService.CreateWithProvider(input.Kind, input.ProviderCode, taskInput)
		}
		if err != nil {
			writeError(w, 400, "invalid_task", err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(t)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	items := s.TaskService.List()
	limit, offset := pageParams(r)
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"tasks": items[offset:end], "total": len(items)})
}

func (s Server) Task(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if strings.HasSuffix(id, "/result") {
		id = strings.TrimSuffix(id, "/result")
		t, ok := s.TaskService.Get(id)
		if !ok {
			writeError(w, 404, "task_not_found", errors.New("task not found"))
			return
		}
		if t.Status != task.StatusSucceeded {
			writeError(w, 409, "result_not_ready", errors.New("task result is not ready"))
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="frameflow-result-`+id+`.json"`)
		writeJSON(w, 200, t)
		return
	}
	if strings.HasSuffix(id, "/cancel") {
		id = strings.TrimSuffix(id, "/cancel")
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		t, err := s.TaskService.Cancel(id)
		if err != nil {
			writeError(w, 409, "task_not_cancellable", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(t)
		return
	}
	if strings.HasSuffix(id, "/retry") {
		id = strings.TrimSuffix(id, "/retry")
		if r.Method != http.MethodPost {
			writeError(w, 405, "method_not_allowed", errors.New("method not allowed"))
			return
		}
		t, err := s.TaskService.Retry(id)
		if err != nil {
			writeError(w, 409, "task_not_retryable", err)
			return
		}
		writeJSON(w, 202, t)
		return
	}
	t, ok := s.Store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodDelete {
		_ = s.Store.Delete(id)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_ = json.NewEncoder(w).Encode(t)
}

func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	return d.Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string, err error) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": err.Error()}})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://127.0.0.1:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func auth(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("Authorization") == "Bearer "+token || r.URL.Query().Get("token") == token
		if token != "" && r.URL.Path != "/health" && !provided {
			writeError(w, 401, "unauthorized", errors.New("valid bearer token required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func pageParams(r *http.Request) (int, int) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (s Server) projects(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]any{"projects": s.Projects.List()})
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, 405, "method_not_allowed", errors.New("method not allowed"))
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, 400, "invalid_request", err)
		return
	}
	id := time.Now().UTC().Format("20060102150405.000000000")
	p, err := s.Projects.Create(id, in.Name)
	if err != nil {
		writeError(w, 400, "invalid_project", err)
		return
	}
	writeJSON(w, 201, p)
}

func (s Server) project(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	if strings.HasSuffix(path, "/drafts") {
		id := strings.TrimSuffix(path, "/drafts")
		var d project.Draft
		if err := decode(r, &d); err != nil {
			writeError(w, 400, "invalid_draft", err)
			return
		}
		if d.ID == "" {
			d.ID = time.Now().UTC().Format("20060102150405.000000000")
		}
		p, err := s.Projects.SaveDraft(id, d)
		if err != nil {
			writeError(w, 404, "project_not_found", err)
			return
		}
		writeJSON(w, 200, p)
		return
	}
	p, ok := s.Projects.Get(path)
	if !ok {
		writeError(w, 404, "project_not_found", errors.New("project not found"))
		return
	}
	writeJSON(w, 200, p)
}

func (s Server) overview(w http.ResponseWriter, r *http.Request) {
	projects := s.Projects.List()
	tasks := s.TaskService.List()
	running := 0
	for _, v := range tasks {
		if v.Status == task.StatusQueued || v.Status == task.StatusRunning {
			running++
		}
	}
	providers := 0
	for _, k := range []provider.Capability{provider.Story, provider.Image, provider.Video} {
		providers += len(s.Providers.List(k))
	}
	materials := 0
	for _, k := range []material.Kind{material.Visual, material.Frame, material.Character, material.Voice, material.Music} {
		materials += len(s.Materials.List(k))
	}
	writeJSON(w, 200, map[string]int{"projects": len(projects), "tasks": len(tasks), "running_tasks": running, "providers": providers, "materials": materials})
}

func (s Server) providers(w http.ResponseWriter, r *http.Request) {
	kind := provider.Capability(r.URL.Query().Get("capability"))
	if r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]any{"providers": s.Providers.List(kind)})
		return
	}
	var input struct {
		provider.Config
		APIKey string `json:"api_key"`
	}
	if err := decode(r, &input); err != nil {
		writeError(w, 400, "invalid_provider", err)
		return
	}
	c := input.Config
	if input.APIKey != "" && s.Vault != nil {
		if err := s.Vault.Put(c.Code, input.APIKey); err != nil {
			writeError(w, 500, "credential_storage_error", err)
			return
		}
		c.CredentialSet = true
	} else if s.Vault != nil {
		c.CredentialSet = s.Vault.Has(c.Code)
	}
	if err := s.Providers.Save(c); err != nil {
		writeError(w, 400, "invalid_provider", err)
		return
	}
	writeJSON(w, 201, c)
}

func (s Server) provider(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/providers/")
	if strings.HasSuffix(path, "/test") {
		code := strings.TrimSuffix(path, "/test")
		c, err := s.Providers.TestConnection(r.Context(), code)
		if err != nil {
			writeError(w, 422, "provider_unavailable", err)
			return
		}
		writeJSON(w, 200, c)
		return
	}
	if r.Method == http.MethodDelete {
		if err := s.Providers.Delete(path); err != nil {
			writeError(w, 500, "delete_failed", err)
			return
		}
		if s.Vault != nil {
			_ = s.Vault.Delete(path)
		}
		w.WriteHeader(204)
		return
	}
	if r.Method == http.MethodPut {
		var c provider.Config
		if err := decode(r, &c); err != nil {
			writeError(w, 400, "invalid_provider", err)
			return
		}
		c.Code = path
		if err := s.Providers.Save(c); err != nil {
			writeError(w, 400, "invalid_provider", err)
			return
		}
		writeJSON(w, 200, c)
		return
	}
	c, ok := s.Providers.Get(path)
	if !ok {
		writeError(w, 404, "provider_not_found", errors.New("provider not found"))
		return
	}
	writeJSON(w, 200, c)
}

func (s Server) materials(w http.ResponseWriter, r *http.Request) {
	kind := material.Kind(r.URL.Query().Get("kind"))
	if r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]any{"materials": s.Materials.List(kind)})
		return
	}
	var a material.Asset
	if err := decode(r, &a); err != nil {
		writeError(w, 400, "invalid_material", err)
		return
	}
	if a.ID == "" {
		a.ID = time.Now().UTC().Format("20060102150405.000000000")
	}
	if err := s.Materials.Save(a); err != nil {
		writeError(w, 400, "invalid_material", err)
		return
	}
	writeJSON(w, 201, a)
}

func (s Server) material(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/materials/")
	if id == "upload" {
		s.uploadMaterial(w, r)
		return
	}
	if r.Method == http.MethodDelete {
		_ = s.Materials.Delete(id)
		w.WriteHeader(204)
		return
	}
	if r.Method == http.MethodPut {
		var a material.Asset
		if err := decode(r, &a); err != nil {
			writeError(w, 400, "invalid_material", err)
			return
		}
		a.ID = id
		if err := s.Materials.Save(a); err != nil {
			writeError(w, 400, "invalid_material", err)
			return
		}
		writeJSON(w, 200, a)
		return
	}
	a, ok := s.Materials.Get(id)
	if !ok {
		writeError(w, 404, "material_not_found", errors.New("material not found"))
		return
	}
	writeJSON(w, 200, a)
}

func (s Server) uploadMaterial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method_not_allowed", errors.New("method not allowed"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 21<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, 413, "upload_too_large", errors.New("upload must be 20 MB or smaller"))
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "file_required", err)
		return
	}
	defer file.Close()
	if header.Size > 20<<20 {
		writeError(w, 413, "upload_too_large", errors.New("upload must be 20 MB or smaller"))
		return
	}
	sample := make([]byte, 512)
	n, readErr := file.Read(sample)
	if readErr != nil && readErr != io.EOF {
		writeError(w, 400, "invalid_media", readErr)
		return
	}
	contentType := http.DetectContentType(sample[:n])
	if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "audio/") && !strings.HasPrefix(contentType, "video/") {
		writeError(w, 415, "unsupported_media_type", errors.New("upload must be an image, audio, or video file"))
		return
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		writeError(w, 400, "invalid_media", err)
		return
	}
	kind := material.Kind(r.FormValue("kind"))
	id := time.Now().UTC().Format("20060102150405.000000000")
	name := filepath.Base(header.Filename)
	if name == "." || name == "" {
		writeError(w, 400, "invalid_filename", errors.New("invalid filename"))
		return
	}
	if err = os.MkdirAll(s.UploadDir, 0750); err != nil {
		writeError(w, 500, "storage_error", err)
		return
	}
	stored := id + filepath.Ext(name)
	path := filepath.Join(s.UploadDir, stored)
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
	if err != nil {
		writeError(w, 500, "storage_error", err)
		return
	}
	_, copyErr := io.Copy(out, file)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if copyErr != nil {
			writeError(w, 500, "storage_error", copyErr)
		} else {
			writeError(w, 500, "storage_error", closeErr)
		}
		return
	}
	asset, err := material.New(id, r.FormValue("name"), kind, time.Now().UTC())
	if err != nil {
		_ = os.Remove(path)
		writeError(w, 400, "invalid_material", err)
		return
	}
	asset.URL = "/media/" + stored
	if err = s.Materials.Save(asset); err != nil {
		_ = os.Remove(path)
		writeError(w, 500, "storage_error", err)
		return
	}
	writeJSON(w, 201, asset)
}
