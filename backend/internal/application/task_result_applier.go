package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type TaskResultApplier struct {
	Projects project.Repository
}

func (a TaskResultApplier) Apply(ctx context.Context, item task.Task) error {
	if a.Projects == nil {
		return errors.New("project repository is required to apply task result")
	}
	if err := item.ValidateScope(); err != nil {
		return err
	}
	output := project.DraftOutput{
		TaskID: item.ID,
		Kind:   string(item.Kind),
		Text:   item.ResultText,
		URL:    item.ResultURL,
	}
	now := time.Now().UTC()
	change := func(value *project.Project) error {
		if item.Kind == task.KindComposition {
			if item.Composition == nil || !strings.HasPrefix(item.ResultURL, "/media/") {
				return errors.New("composition result must be saved locally")
			}
			for i := range value.Drafts {
				draft := &value.Drafts[i]
				if draft.ID != item.DraftID {
					continue
				}
				for _, existing := range draft.Compositions {
					if existing.TaskID == item.ID {
						return nil
					}
				}
				clipIDs := make([]string, 0, len(item.Composition.Clips))
				for _, clip := range item.Composition.Clips {
					clipIDs = append(clipIDs, clip.VersionID)
				}
				draft.Compositions = append(draft.Compositions, project.Composition{
					ID: "composition-" + item.ID, TaskID: item.ID, ClipVersionIDs: clipIDs,
					MusicMaterialID: item.Composition.MusicMaterialID, MusicVolume: item.Composition.MusicVolume,
					SourceVolume: item.Composition.SourceVolume, VoiceVolume: item.Composition.VoiceVolume,
					AspectRatio: item.Composition.AspectRatio, Resolution: item.Composition.Resolution,
					LocalMediaPath: item.ResultURL, InputFingerprint: item.InputHash, CreatedAt: now,
				})
				return nil
			}
			return nil
		}
		if item.Kind == task.KindVideo && item.SceneID != "" {
			for i := range value.Drafts {
				draft := &value.Drafts[i]
				if draft.ID != item.DraftID {
					continue
				}
				if !draft.HasScene(item.SceneID) {
					return nil
				}
				for _, version := range draft.VideoVersions {
					if version.TaskID == item.ID {
						return nil
					}
				}
				if !strings.HasPrefix(item.ResultURL, "/media/") {
					return errors.New("video result must be saved locally before version creation")
				}
				return draft.AddVideoVersion(project.VideoVersion{
					ID: "version-" + item.ID, SceneID: item.SceneID, TaskID: item.ID,
					LocalMediaPath: item.ResultURL, DurationSeconds: float64(item.Input.Duration),
					InputFingerprint: item.InputHash, CreatedAt: now,
				})
			}
			return nil
		}
		if item.ResultCandidate != nil {
			for i := range value.Drafts {
				draft := &value.Drafts[i]
				if draft.ID != item.DraftID {
					continue
				}
				for _, existing := range draft.Candidates {
					if existing.ID == item.ResultCandidate.ID {
						return nil
					}
				}
				return draft.AddCandidate(*item.ResultCandidate)
			}
			return errors.New("draft not found in project")
		}
		return value.ApplyTaskResult(item.DraftID, output, now)
	}
	if atomic, ok := a.Projects.(project.AtomicRepository); ok {
		_, err := atomic.Update(ctx, item.ProjectID, change)
		return err
	}
	value, err := a.Projects.Get(ctx, item.ProjectID)
	if err != nil {
		return err
	}
	if err = change(&value); err != nil {
		return err
	}
	return a.Projects.Save(ctx, value)
}
