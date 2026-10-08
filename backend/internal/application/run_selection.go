package application

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
	"sort"
	"time"
)

func (s TaskService) runSteps(ctx context.Context, r task.GenerationRun) []task.Task {
	steps := []task.Task{}
	for _, id := range r.TaskIDs {
		if t, err := s.Store.Get(ctx, id); err == nil {
			steps = append(steps, t)
		}
	}
	return steps
}

func (s TaskService) validRunScope(g *gin.Context) bool {
	p, err := s.Projects.Get(g.Request.Context(), g.Param("id"))
	if err == nil {
		if d, e := draftByID(&p, g.Param("draftId")); e == nil {
			for _, r := range d.GenerationRuns {
				if r.ID == g.Param("runId") && r.SceneID == g.Param("sceneId") {
					return true
				}
			}
		}
	}
	writeError(g, Invalid("run_not_found", errors.New("任务不属于当前分镜或已不存在")))
	return false
}

func (s TaskService) ListGenerationRuns(g *gin.Context) {
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
	if !d.HasScene(g.Param("sceneId")) {
		writeError(g, Invalid("run_not_found", errors.New("分镜不存在")))
		return
	}
	runs := []task.GenerationRun{}
	for _, r := range d.GenerationRuns {
		if r.SceneID == g.Param("sceneId") {
			runs = append(runs, r)
		}
	}
	sort.SliceStable(runs, func(i, j int) bool {
		if runs[i].CreatedAt == nil {
			return runs[j].CreatedAt != nil
		}
		if runs[j].CreatedAt == nil {
			return false
		}
		return runs[i].CreatedAt.Before(*runs[j].CreatedAt)
	})
	g.JSON(200, gin.H{"runs": runs})
}

func (s TaskService) SelectRunMedia(g *gin.Context) {
	if !s.validRunScope(g) {
		return
	}
	var in struct {
		Expected       *int64 `json:"expected_version"`
		TakeID         string `json:"take_id"`
		UtteranceID    string `json:"utterance_id"`
		VideoVersionID string `json:"video_version_id"`
	}
	if err := g.ShouldBindJSON(&in); err != nil || in.Expected == nil || (in.TakeID == "") == (in.VideoVersionID == "") {
		writeError(g, Invalid("run_result_mismatch", errors.New("请选择一个本任务结果")))
		return
	}
	p, scopeErr := s.Projects.Get(g.Request.Context(), g.Param("id"))
	if scopeErr != nil {
		writeError(g, scopeErr)
		return
	}
	d, scopeErr := draftByID(&p, g.Param("draftId"))
	if scopeErr != nil {
		writeError(g, scopeErr)
		return
	}
	taskID := ""
	for _, take := range d.MediaTakes {
		if take.ID == in.TakeID {
			taskID = take.TaskID
		}
	}
	for _, v := range d.VideoVersions {
		if v.ID == in.VideoVersionID {
			taskID = v.TaskID
		}
	}
	selectedTask, taskErr := s.Store.Get(g.Request.Context(), taskID)
	if taskErr != nil || selectedTask.RunID != g.Param("runId") || selectedTask.SceneID != g.Param("sceneId") || selectedTask.Status != task.StatusSucceeded {
		writeError(g, Invalid("run_result_mismatch", errors.New("结果不属于当前任务或尚未完成")))
		return
	}
	generationRunTransition.Lock()
	var result task.GenerationRun
	_, err := s.Projects.(project.AtomicRepository).Update(g.Request.Context(), g.Param("id"), func(p *project.Project) error {
		d, e := draftByID(p, g.Param("draftId"))
		if e != nil {
			return e
		}
		if d.Version != *in.Expected {
			return project.ErrVersionConflict
		}
		for i := range d.GenerationRuns {
			r := &d.GenerationRuns[i]
			if r.ID != g.Param("runId") || r.SceneID != g.Param("sceneId") {
				continue
			}
			if r.State == "cancelled" {
				return errors.New("任务已取消")
			}
			if r.Selection == nil {
				return errors.New("旧任务缺少独立选用记录，请在旧流程处理或重新生成")
			}
			if in.TakeID != "" {
				found := false
				for _, take := range d.MediaTakes {
					if take.ID != in.TakeID || take.SceneID != r.SceneID {
						continue
					}
					if selectedTask.ID != take.TaskID || selectedTask.RunID != r.ID {
						return errors.New("结果不属于当前任务或尚未完成")
					}
					if take.Kind == "image" {
						if in.UtteranceID != "" {
							return errors.New("图片不能指定台词")
						}
						if len(r.Plan) > 0 && isVideoPath(r.Plan[len(r.Plan)-1].Path) {
							return errors.New("视频已生成，不能更换此任务首帧")
						}
						r.Selection.ImageTakeID = take.ID
					} else if take.Kind == "speech" && take.UtteranceID == in.UtteranceID {
						if r.Selection.SpeechTakeIDs == nil {
							r.Selection.SpeechTakeIDs = map[string]string{}
						}
						r.Selection.SpeechTakeIDs[in.UtteranceID] = take.ID
					} else {
						return errors.New("结果类型或台词不匹配")
					}
					found = true
					break
				}
				if !found {
					return errors.New("结果不存在")
				}
			} else {
				found := false
				for _, v := range d.VideoVersions {
					if v.ID != in.VideoVersionID || v.SceneID != r.SceneID {
						continue
					}
					if selectedTask.ID != v.TaskID || selectedTask.RunID != r.ID {
						return errors.New("视频不属于当前任务或尚未完成")
					}
					r.Selection.VideoVersionID = v.ID
					if d.SelectedVersions == nil {
						d.SelectedVersions = map[string]string{}
					}
					d.SelectedVersions[r.SceneID] = v.ID
					// Publish only this run's selected audio for the existing composition pipeline.
					if d.SelectedMedia == nil {
						d.SelectedMedia = map[string]string{}
					}
					for u, take := range r.Selection.SpeechTakeIDs {
						d.SelectedMedia[r.SceneID+":speech:"+u] = take
					}
					found = true
					break
				}
				if !found {
					return errors.New("视频版本不存在")
				}
			}
			updated := time.Now().UTC()
			r.UpdatedAt = &updated
			d.Version++
			result = *r
			return nil
		}
		return errors.New("任务不存在")
	})
	generationRunTransition.Unlock()
	if err != nil {
		if errors.Is(err, project.ErrVersionConflict) {
			writeError(g, Conflict("draft_version_conflict", err))
		} else {
			writeError(g, Invalid("run_result_mismatch", err))
		}
		return
	}
	_ = s.advanceRun(g.Request.Context(), g.Param("id"), g.Param("draftId"), result.ID)
	s.GetGenerationRun(g)
}

// Compile the video from the immutable run input and its own chosen first frame.
func (s TaskService) frozenRunVideo(ctx context.Context, d project.Draft, r task.GenerationRun) (task.GenerationSnapshot, error) {
	base := r.Plan[0]
	model, err := s.Providers.Catalog.Models.GetModel(ctx, r.VideoModelID)
	if err != nil {
		return base, err
	}
	cap, ok := provider.InputCapabilityFor(model.RemoteID)
	if !ok || !model.Enabled || !model.Supported || !model.Supports(provider.Video) || !cap.FirstFrame {
		return base, errors.New("视频模型不可用")
	}
	compiler := GenerationCompiler{Materials: s.Materials, Directory: s.UploadDir}
	for _, take := range d.MediaTakes {
		if take.ID != r.Selection.ImageTakeID {
			continue
		}
		item, err := s.Store.Get(ctx, take.TaskID)
		if err != nil || item.RunID != r.ID || take.Kind != "image" {
			return base, errors.New("首帧不属于当前任务")
		}
		ref, err := compiler.reference(ctx, take.MaterialID, "first_frame")
		if err != nil {
			return base, err
		}
		base.ID = ""
		base.InputHash = ""
		base.ModelID = r.VideoModelID
		base.CapabilityVersion = cap.Version
		base.Path = "first_frame_video"
		base.Resolution = r.Resolution
		base.DurationSeconds = r.DurationSeconds
		base.CreatedAt = time.Now().UTC()
		base.Entities = []task.SnapshotEntity{{BindingID: "first_frame", EntityID: "first_frame", References: []task.MediaReference{ref}}}
		return task.FreezeSnapshot(base)
	}
	return base, errors.New("请选用本任务的首帧")
}
