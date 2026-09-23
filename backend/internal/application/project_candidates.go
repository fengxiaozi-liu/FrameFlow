package application

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/gin-gonic/gin"
)

var errCandidateNeedsReview = errors.New("candidate is based on older draft content; review and reconfirm")

func draftByID(value *project.Project, id string) (*project.Draft, error) {
	for i := range value.Drafts {
		if value.Drafts[i].ID == id {
			return &value.Drafts[i], nil
		}
	}
	return nil, errors.New("draft not found in project")
}

func (s ProjectService) GetDraft(ctx *gin.Context) {
	p, err := s.Repo.Get(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	d, err := draftByID(&p, ctx.Param("draftId"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, d)
}

func (s ProjectService) ListCandidates(ctx *gin.Context) {
 p,err:=s.Repo.Get(ctx.Request.Context(),ctx.Param("id"))
 if err!=nil {writeError(ctx,err);return}
 d,err:=draftByID(&p,ctx.Param("draftId"))
 if err!=nil {writeError(ctx,err);return}
 values:=d.Candidates
 if values==nil {values=[]story.Candidate{}}
 ctx.JSON(http.StatusOK,gin.H{"candidates":values})
}

func (s ProjectService) GetCandidate(ctx *gin.Context) {
 p,err:=s.Repo.Get(ctx.Request.Context(),ctx.Param("id"))
 if err!=nil {writeError(ctx,err);return}
 d,err:=draftByID(&p,ctx.Param("draftId"))
 if err!=nil {writeError(ctx,err);return}
 for _,candidate:=range d.Candidates {
  if candidate.ID==ctx.Param("candidateId") {ctx.JSON(http.StatusOK,candidate);return}
 }
 writeError(ctx,Invalid("candidate_not_found",errors.New("candidate not found")))
}

func (s ProjectService) ApplyCandidate(ctx *gin.Context) {
	var input struct {
		ExpectedVersion *int64 `json:"expected_version"`
		Action          string `json:"action"`
		Reconfirm       bool   `json:"reconfirm"`
	}
	if err := ctx.ShouldBindJSON(&input); err != nil || input.ExpectedVersion == nil || *input.ExpectedVersion < 0 {
		writeError(ctx, Invalid("invalid_candidate_apply", errors.New("expected_version is required")))
		return
	}
	var result project.Draft
	atomic, ok := s.Repo.(project.AtomicRepository)
	if !ok {
		writeError(ctx, errors.New("atomic project repository is required"))
		return
	}
	_, err := atomic.Update(ctx.Request.Context(), ctx.Param("id"), func(p *project.Project) error {
		d, findErr := draftByID(p, ctx.Param("draftId"))
		if findErr != nil {
			return findErr
		}
		result = *d
		if d.Version != *input.ExpectedVersion {
			return project.ErrVersionConflict
		}
		var candidate *story.Candidate
		for i := range d.Candidates {
			if d.Candidates[i].ID == ctx.Param("candidateId") {
				candidate = &d.Candidates[i]
				break
			}
		}
		if candidate == nil || candidate.Status != "preview" {
			return errors.New("candidate preview not found")
		}
		if (candidate.SourceVersion != d.Version || candidate.Target == story.CandidateStoryboard && candidate.SourceBody != d.Story.Body) && !input.Reconfirm {
			return errCandidateNeedsReview
		}
		now := time.Now().UTC()
		switch candidate.Target {
		case story.CandidateStory:
			if input.Action != "append" && input.Action != "replace" {
				return errors.New("story candidate action must be append or replace")
			}
			previous := d.Story.Body
			if input.Action == "append" && strings.TrimSpace(previous) != "" {
				d.Story.Body = previous + "\n\n" + candidate.Body
			} else {
				d.Story.Body = candidate.Body
			}
			d.BodySnapshots = append(d.BodySnapshots, project.BodySnapshot{ID: "body-" + candidate.ID, Body: previous, AppliedBody: d.Story.Body, CreatedAt: now})
		case story.CandidateStoryboard:
			if input.Action != "replace" {
				return errors.New("storyboard candidate requires replace action")
			}
			if err := d.CaptureStoryboardSnapshot("board-"+candidate.ID, now); err != nil {
				return err
			}
			d.Story.Scenes = nil
			for _, generated := range candidate.Scenes {
				generated.ID = ""
				if err := d.Story.AddScene(generated); err != nil {
					return err
				}
			}
			d.Bindings, d.VideoVersions, d.SelectedVersions = nil, nil, nil
			d.StoryboardSourceBody = d.Story.Body
		default:
			return errors.New("unsupported candidate target")
		}
		candidate.Status = "applied"
		d.Story.UpdatedAt, d.UpdatedAt = now, now
		d.Version++
		result = *d
		return nil
	})
	if errors.Is(err, project.ErrVersionConflict) || errors.Is(err, errCandidateNeedsReview) {
		ctx.JSON(http.StatusConflict, gin.H{"code": "draft_version_conflict", "message": err.Error(), "draft": result})
		return
	}
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (s ProjectService) RestoreDraft(ctx *gin.Context) {
	var input struct {
		ExpectedVersion *int64 `json:"expected_version"`
		SnapshotID      string `json:"snapshot_id"`
		Reconfirm       bool   `json:"reconfirm"`
	}
	if err := ctx.ShouldBindJSON(&input); err != nil || input.ExpectedVersion == nil || input.SnapshotID == "" {
		writeError(ctx, Invalid("invalid_restore", errors.New("expected_version and snapshot_id are required")))
		return
	}
	var restored project.Draft
	atomic, ok := s.Repo.(project.AtomicRepository)
	if !ok {
		writeError(ctx, errors.New("atomic project repository is required"))
		return
	}
	_, err := atomic.Update(ctx.Request.Context(), ctx.Param("id"), func(p *project.Project) error {
		d, findErr := draftByID(p, ctx.Param("draftId"))
		if findErr != nil {
			return findErr
		}
		restored = *d
		if d.Version != *input.ExpectedVersion {
			return project.ErrVersionConflict
		}
		for _, snapshot := range d.BodySnapshots {
			if snapshot.ID == input.SnapshotID {
				if d.Story.Body != snapshot.AppliedBody && !input.Reconfirm {
					return errCandidateNeedsReview
				}
				d.Story.Body = snapshot.Body
				d.Version++
				d.UpdatedAt = time.Now().UTC()
				restored = *d
				return nil
			}
		}
		for _, snapshot := range d.StoryboardSnapshots {
			if snapshot.ID == input.SnapshotID {
				d.Story.Scenes = append([]story.Scene(nil), snapshot.Scenes...)
				d.StoryboardSourceBody = snapshot.SourceBody
				d.Bindings = append([]project.SceneBinding(nil), snapshot.Bindings...)
				d.VideoVersions = append([]project.VideoVersion(nil), snapshot.VideoVersions...)
				d.SelectedVersions = make(map[string]string, len(snapshot.SelectedVersions))
				for sceneID, versionID := range snapshot.SelectedVersions {
					d.SelectedVersions[sceneID] = versionID
				}
				d.Version++
				d.UpdatedAt = time.Now().UTC()
				restored = *d
				return nil
			}
		}
		return errors.New("snapshot not found")
	})
	if errors.Is(err, project.ErrVersionConflict) || errors.Is(err, errCandidateNeedsReview) {
		ctx.JSON(http.StatusConflict, gin.H{"code": "draft_version_conflict", "message": err.Error(), "draft": restored})
		return
	}
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, restored)
}
