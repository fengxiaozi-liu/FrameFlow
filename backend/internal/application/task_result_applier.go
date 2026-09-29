package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type TaskResultApplier struct {
	Projects  project.Repository
	Materials material.Repository
	Directory string
	Probe     func(context.Context, string) (task.MediaProbeResult, error)
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
	var take *project.MediaTake
	if item.Snapshot != nil && (item.Kind == task.KindImage || item.Kind == task.KindSpeech) {
		if a.Materials == nil {
			return errors.New("material repository required")
		}
		path, err := localMediaPath(a.Directory, item.ResultURL)
		if err != nil {
			return err
		}
		asset := material.Asset{ID: "result-" + item.ID, Name: "生成结果 " + item.ID, Kind: material.Scene, URL: item.ResultURL, CreatedAt: now}
		uID := ""
		duration := 0.0
		if item.Kind == task.KindSpeech {
			asset.Kind = material.Voice
			if len(item.Snapshot.Utterances) != 1 {
				return errors.New("speech utterance missing")
			}
			uID = item.Snapshot.Utterances[0].ID
			if a.Probe == nil {
				return errors.New("speech duration probe required")
			}
			p, err := a.Probe(ctx, path)
			if err != nil || !p.HasAudio {
				return errors.New("speech result cannot be probed")
			}
			duration = p.Duration
			asset.Media.DurationSeconds = duration
		}
		if _, err := a.Materials.Get(ctx, asset.ID); err != nil {
			if err = a.Materials.Save(ctx, asset); err != nil {
				return err
			}
		}
		take = &project.MediaTake{ID: "take-" + item.ID, Kind: string(item.Kind), SceneID: item.SceneID, UtteranceID: uID, TaskID: item.ID, MaterialID: asset.ID, SnapshotID: item.Snapshot.ID, InputHash: item.InputHash, DependencyHash: item.Snapshot.DependencyHash, DurationSeconds: duration}
	}
	change := func(value *project.Project) error {
		if take != nil {
			for i := range value.Drafts {
				d := &value.Drafts[i]
				if d.ID != item.DraftID {
					continue
				}
				for _, old := range d.MediaTakes {
					if old.TaskID == item.ID {
						return nil
					}
				}
				for _, scene := range d.Story.Scenes {
					if scene.ID == item.SceneID {
						take.Stale = dependencyHash(value.EntityCatalog, scene, item.Snapshot.Path, take.UtteranceID) != take.DependencyHash
						d.MediaTakes = append(d.MediaTakes, *take)
						return nil
					}
				}
			}
			return nil
		}
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
