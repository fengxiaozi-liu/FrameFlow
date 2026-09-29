package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
	"sync"
	"time"
)

var generationRunTransition sync.Mutex

func runIdentity(parts string) string {
	h := sha256.Sum256([]byte(parts))
	return hex.EncodeToString(h[:16])
}
func (s TaskService) CreateGenerationRun(g *gin.Context) {
	var in GenerationRequest
	if err := g.ShouldBindJSON(&in); err != nil || in.IdempotencyKey == "" || in.VideoModelID == "" || in.Path != "reference_image_then_video" {
		writeError(g, Invalid("invalid_run", errors.New("generation input and video_model_id required")))
		return
	}
	id := "run-" + runIdentity(g.Param("id")+g.Param("draftId")+g.Param("sceneId")+in.IdempotencyKey)
	b, _ := json.Marshal(in)
	hash := runIdentity(string(b))
	p, err := s.Projects.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	d, err := draftByID(&p, g.Param("draftId"))
	if err != nil {
		writeError(g, err)
		return
	}
	for _, r := range d.GenerationRuns {
		if r.ID == id {
			if r.RequestHash != hash {
				writeError(g, Conflict("idempotency_key_reused", errors.New("run inputs changed")))
				return
			}
			g.JSON(202, r)
			return
		}
	}
	model, modelErr := s.Providers.Catalog.Models.GetModel(g.Request.Context(), in.VideoModelID)
	cap, known := provider.InputCapabilityFor(model.RemoteID)
	if modelErr != nil || !known || !model.Enabled || !model.Supported || !model.Supports(provider.Video) || !cap.FirstFrame {
		writeError(g, Invalid("invalid_video_model", errors.New("请选择可用的首帧视频模型后开始制作")))
		return
	}
	duration := in.DurationSeconds
	if duration == 0 {
		for _, scene := range d.Story.Scenes {
			if scene.ID == g.Param("sceneId") {
				duration = scene.DurationSeconds
			}
		}
	}
	resolution := in.Resolution
	if resolution == "" {
		resolution = "720P"
	}
	validDuration, validResolution := false, false
	for _, value := range cap.Durations {
		validDuration = validDuration || value == duration
	}
	for _, value := range cap.Resolutions {
		validResolution = validResolution || value == resolution
	}
	if !validDuration || !validResolution {
		writeError(g, Invalid("invalid_video_options", errors.New("视频时长或清晰度不受所选模型支持")))
		return
	}
	snap, err := s.compileShot(g, in)
	if err != nil {
		writeError(g, Invalid("invalid_run", err))
		return
	}
	run := task.GenerationRun{ID: id, SceneID: g.Param("sceneId"), State: "preparing", SnapshotID: snap.ID, RequestHash: hash, VideoModelID: in.VideoModelID, DurationSeconds: in.DurationSeconds, Resolution: in.Resolution, Plan: []task.GenerationSnapshot{snap}, TaskIDs: []string{}}
	for _, scene := range d.Story.Scenes {
		if scene.ID == run.SceneID {
			r, err := ResolveBindings(p.EntityCatalog, scene, true)
			if err != nil {
				writeError(g, Invalid("invalid_run", err))
				return
			}
			for _, u := range scene.Utterances {
				speechIn := in
				speechIn.ModelID = r.Voices[u.ID].ModelID
				speechIn.Path = "speech"
				speechIn.UtteranceID = u.ID
				v, err := s.compileShot(g, speechIn)
				if err != nil {
					writeError(g, Invalid("invalid_run", err))
					return
				}
				run.Plan = append(run.Plan, v)
			}
		}
	}
	atomic, ok := s.Projects.(project.AtomicRepository)
	if !ok {
		writeError(g, errors.New("atomic repository required"))
		return
	}
	_, err = atomic.Update(g.Request.Context(), p.ID, func(current *project.Project) error {
		draft, err := draftByID(current, d.ID)
		if err != nil {
			return err
		}
		for _, old := range draft.GenerationRuns {
			if old.ID == id {
				if old.RequestHash != hash {
					return errors.New("run idempotency conflict")
				}
				run = old
				return nil
			}
		}
		if in.ExpectedVersion == nil || draft.Version != *in.ExpectedVersion {
			return project.ErrVersionConflict
		}
		draft.GenerationRuns = append(draft.GenerationRuns, run)
		return nil
	})
	if err != nil {
		writeError(g, Conflict("run_conflict", err))
		return
	}
	_ = s.advanceRun(g.Request.Context(), p.ID, d.ID, id)
	g.JSON(202, run)
}
func (s TaskService) GetGenerationRun(g *gin.Context) {
	_ = s.advanceRun(g.Request.Context(), g.Param("id"), g.Param("draftId"), g.Param("runId"))
	p, err := s.Projects.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	d, err := draftByID(&p, g.Param("draftId"))
	if err == nil {
		for _, r := range d.GenerationRuns {
			if r.ID == g.Param("runId") {
				g.JSON(200, r)
				return
			}
		}
	}
	writeError(g, Invalid("run_not_found", errors.New("run not found")))
}
func (s TaskService) CancelGenerationRun(g *gin.Context) {
	generationRunTransition.Lock()
	defer generationRunTransition.Unlock()
	atomic := s.Projects.(project.AtomicRepository)
	var run task.GenerationRun
	_, err := atomic.Update(g.Request.Context(), g.Param("id"), func(p *project.Project) error {
		d, err := draftByID(p, g.Param("draftId"))
		if err != nil {
			return err
		}
		for i := range d.GenerationRuns {
			r := &d.GenerationRuns[i]
			if r.ID == g.Param("runId") {
				r.State = "cancelled"
				run = *r
				return nil
			}
		}
		return errors.New("run not found")
	})
	if err != nil {
		writeError(g, err)
		return
	}
	for i := range run.Plan {
		id := fmt.Sprintf("%s-%d", run.ID, i)
		item, err := s.Store.Get(g.Request.Context(), id)
		if err == nil && (item.Status == task.StatusQueued || item.Status == task.StatusRunning) {
			_ = item.Cancel(time.Now().UTC())
			_ = s.Store.Save(g.Request.Context(), item)
			if c, ok := s.Enqueuer.(interface {
				CancelTask(context.Context, string) error
			}); ok {
				_ = c.CancelTask(g.Request.Context(), id)
			}
		}
	}
	g.JSON(200, run)
}
func (s TaskService) AdvanceGenerationRuns(ctx context.Context) {
	p, err := s.Projects.List(ctx)
	if err != nil {
		return
	}
	for _, p := range p {
		for _, d := range p.Drafts {
			for _, r := range d.GenerationRuns {
				if r.State != "completed" && r.State != "cancelled" {
					_ = s.advanceRun(ctx, p.ID, d.ID, r.ID)
				}
			}
		}
	}
}
func (s TaskService) advanceRun(ctx context.Context, pID, dID, rID string) error {
	generationRunTransition.Lock()
	defer generationRunTransition.Unlock()
	p, err := s.Projects.Get(ctx, pID)
	if err != nil {
		return err
	}
	d, err := draftByID(&p, dID)
	if err != nil {
		return err
	}
	var run task.GenerationRun
	for _, r := range d.GenerationRuns {
		if r.ID == rID {
			run = r
		}
	}
	if run.ID == "" || run.State == "cancelled" || run.State == "completed" {
		return nil
	}
	previousLength := len(run.Plan)
	run.TaskIDs = []string{}
	allDone := true
	run.Error = ""
	for i, snap := range run.Plan {
		id := fmt.Sprintf("%s-%d", run.ID, i)
		run.TaskIDs = append(run.TaskIDs, id)
		kind := task.KindImage
		if snap.Path == "speech" {
			kind = task.KindSpeech
		} else if snap.Path == "first_frame_video" {
			kind = task.KindVideo
		}
		item := task.New(id, kind, task.Input{Prompt: snap.Prompt, Duration: snap.DurationSeconds, AspectRatio: snap.AspectRatio, Resolution: snap.Resolution}, time.Now().UTC())
		item.ProjectID, item.DraftID, item.SceneID = pID, dID, run.SceneID
		item.RunID = run.ID
		item.ModelID = snap.ModelID
		item.Snapshot = &snap
		item.InputHash = snap.InputHash
		item.InputVersion = snap.DraftVersion
		if kind == task.KindVideo {
			item.DependsOn = []string{run.ID + "-0"}
		}
		repo := s.Store.(interface {
			CreateIfAbsent(context.Context, task.Task) (task.Task, bool, error)
		})
		stored, created, err := repo.CreateIfAbsent(ctx, item)
		if err != nil {
			return err
		}
		if created && s.Enqueuer != nil {
			if err = s.Enqueuer.Enqueue(ctx, stored); err != nil {
				return s.rejectSubmission(ctx, stored, err)
			}
		}
		if stored.Status != task.StatusSucceeded {
			allDone = false
		}
		if stored.Status == task.StatusFailed || stored.Status == task.StatusCancelled {
			run.State = "failed"
			run.Error = stored.Error
			if run.Error == "" {
				run.Error = "子任务已取消"
			}
		}
	}
	if run.Error == "" {
		run.State = "generating"
		if len(run.Plan) > 0 && run.Plan[len(run.Plan)-1].Path == "first_frame_video" {
			run.State = "generating_video"
		}
		if allDone {
			hasVideo := len(run.Plan) > 0 && run.Plan[len(run.Plan)-1].Path == "first_frame_video"
			if !hasVideo {
				run.State = "awaiting_selection"
				selected := d.SelectedMedia[run.SceneID+":image"] != ""
				for _, snap := range run.Plan {
					if snap.Path == "speech" {
						selected = selected && d.SelectedMedia[run.SceneID+":speech:"+snap.Utterances[0].ID] != ""
					}
				}
				if selected {
					for _, scene := range d.Story.Scenes {
						if scene.ID == run.SceneID {
							if dependencyHash(p.EntityCatalog, scene, run.Plan[0].Path, "") != run.Plan[0].DependencyHash {
								run.State = "failed"
								run.Error = "镜头设定已改变，请创建新流程"
								break
							}
							v := d.Version
							input := GenerationRequest{ExpectedVersion: &v, ModelID: run.VideoModelID, Path: "first_frame_video", AspectRatio: run.Plan[0].AspectRatio, DurationSeconds: run.DurationSeconds, Resolution: run.Resolution}
							snap, err := (GenerationCompiler{Models: s.Providers.Catalog.Models, Materials: s.Materials, Directory: s.UploadDir}).Compile(ctx, p, *d, scene, input)
							if err != nil {
								run.State = "failed"
								run.Error = err.Error()
							} else {
								run.Plan = append(run.Plan, snap)
								run.State = "generating_video"
							}
						}
					}
				}
			} else {
				run.State = "awaiting_video_selection"
				lastTask := fmt.Sprintf("%s-%d", run.ID, len(run.Plan)-1)
				version := "version-" + lastTask
				if d.SelectedVersions[run.SceneID] == version {
					run.State = "awaiting_composition"
					for _, c := range d.Compositions {
						for _, v := range c.ClipVersionIDs {
							if v == version {
								run.State = "completed"
							}
						}
					}
				}
			}
		}
	}
	_, err = s.Projects.(project.AtomicRepository).Update(ctx, pID, func(current *project.Project) error {
		draft, err := draftByID(current, dID)
		if err != nil {
			return err
		}
		for i := range draft.GenerationRuns {
			r := &draft.GenerationRuns[i]
			if r.ID == rID && r.State != "cancelled" && len(r.Plan) == previousLength {
				*r = run
				return nil
			}
		}
		return nil
	})
	return err
}
