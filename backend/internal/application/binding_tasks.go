package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
	"time"
)

func (s TaskService) CreateSceneImage(g *gin.Context)  { s.createBindingTask(g, task.KindImage) }
func (s TaskService) CreateSceneSpeech(g *gin.Context) { s.createBindingTask(g, task.KindSpeech) }
func (s TaskService) CreateBoundVideo(g *gin.Context)  { s.createBindingTask(g, task.KindVideo) }
func (s TaskService) createBindingTask(g *gin.Context, kind task.Kind) {
	var in GenerationRequest
	if err := g.ShouldBindJSON(&in); err != nil {
		writeError(g, Invalid("invalid_generation", err))
		return
	}
	if kind == task.KindSpeech {
		in.UtteranceID = g.Param("utteranceId")
	}
	expectedPath := map[task.Kind]string{task.KindImage: "reference_image_then_video", task.KindVideo: "first_frame_video", task.KindSpeech: "speech"}[kind]
	if in.Path != expectedPath || in.IdempotencyKey == "" || in.ExpectedVersion == nil {
		writeError(g, Invalid("invalid_generation", errors.New("path, expected_version and idempotency_key required")))
		return
	}
	requestBytes, _ := json.Marshal(in)
	requestDigest := sha256.Sum256(requestBytes)
	identity := sha256.Sum256([]byte(g.Param("id") + "\x00" + g.Param("draftId") + "\x00" + g.Param("sceneId") + "\x00" + string(kind) + "\x00" + in.IdempotencyKey))
	id := "binding-" + hex.EncodeToString(identity[:16])
	fingerprint := hex.EncodeToString(requestDigest[:])
	if existing, err := s.Store.Get(g.Request.Context(), id); err == nil {
		if existing.RequestHash != fingerprint {
			writeError(g, Conflict("idempotency_key_reused", errors.New("request parameters changed; use a new idempotency key")))
			return
		}
		g.JSON(202, existing)
		return
	}
	snap, err := s.compileShot(g, in)
	if errors.Is(err, project.ErrVersionConflict) {
		writeError(g, Conflict("draft_version_conflict", err))
		return
	}
	if err != nil {
		writeError(g, Invalid("invalid_generation", err))
		return
	}
	item := task.New(id, kind, task.Input{Prompt: snap.Prompt, AspectRatio: snap.AspectRatio, Duration: snap.DurationSeconds, Resolution: snap.Resolution}, time.Now().UTC())
	item.ProjectID, item.DraftID, item.SceneID = snap.ProjectID, snap.DraftID, snap.SceneID
	item.ModelID = snap.ModelID
	item.ModelConfig = snap.ModelConfig
	item.ConnectionConfig = snap.ConnectionConfig
	item.InputVersion = snap.DraftVersion
	item.InputHash = snap.InputHash
	item.Snapshot = &snap
	item.IdempotencyKey = in.IdempotencyKey
	item.RequestHash = fingerprint
	repo, ok := s.Store.(interface {
		CreateIfAbsent(context.Context, task.Task) (task.Task, bool, error)
	})
	if !ok {
		writeError(g, errors.New("atomic task repository required"))
		return
	}
	item, created, err := repo.CreateIfAbsent(g.Request.Context(), item)
	if err != nil {
		writeError(g, err)
		return
	}
	if item.RequestHash != fingerprint {
		writeError(g, Conflict("idempotency_key_reused", errors.New("request changed")))
		return
	}
	if created {
		s.broadcast(g.Request.Context(), item)
		if s.Enqueuer != nil {
			if err = s.Enqueuer.Enqueue(g.Request.Context(), item); err != nil {
				writeError(g, s.rejectSubmission(g.Request.Context(), item, err))
				return
			}
		}
	}
	g.JSON(202, item)
}
