package project

import (
	"errors"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

// RemoveFailedRun removes history only; selected media and generated files stay available.
func (p *Project) RemoveFailedRun(draftID, sceneID, runID string, children []task.Task, now time.Time) error {
	for di := range p.Drafts {
		d := &p.Drafts[di]
		if d.ID != draftID {
			continue
		}
		for ri, run := range d.GenerationRuns {
			if run.ID != runID || run.SceneID != sceneID {
				continue
			}
			if run.State != "failed" {
				return &fault.Error{Kind: "conflict", Code: "run_not_deletable", Cause: errors.New("仅支持删除失败的制作任务，请刷新状态后重试")}
			}
			for _, child := range children {
				if child.Status == task.StatusQueued || child.Status == task.StatusRunning {
					return &fault.Error{Kind: "conflict", Code: "run_in_use", Cause: errors.New("该制作任务仍有排队或执行中的步骤，请完成或取消后再删除")}
				}
			}
			d.GenerationRuns = append(d.GenerationRuns[:ri], d.GenerationRuns[ri+1:]...)
			// Composition must retain the source-audio policy after task history is removed.
			for vi := range d.VideoVersions {
				version := &d.VideoVersions[vi]
				if version.GenerationPath != "" {
					continue
				}
				for _, child := range children {
					if child.ID == version.TaskID && child.Snapshot != nil {
						version.GenerationPath = child.Snapshot.Path
						break
					}
				}
			}
			d.Version++
			d.UpdatedAt = now
			return nil
		}
	}
	return fault.ErrNotFound
}
