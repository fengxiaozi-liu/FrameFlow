package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
	"strings"
	"time"
	"unicode/utf8"
)

type ReferenceImageRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	Prompt         string `json:"prompt"`
	ModelID        string `json:"model_id"`
	AspectRatio    string `json:"aspect_ratio"`
}

func (s TaskService) CreateReferenceImage(g *gin.Context) {
	ctx := g.Request.Context()
	var in ReferenceImageRequest
	if err := g.ShouldBindJSON(&in); err != nil {
		writeError(g, Invalid("invalid_reference_image", err))
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Prompt = strings.TrimSpace(in.Prompt)
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > 128 || in.ModelID == "" || in.Name == "" || utf8.RuneCountInString(in.Name) > 100 || in.Prompt == "" || utf8.RuneCountInString(in.Prompt) > 4000 || (in.Kind != "scene" && in.Kind != "character") || (in.AspectRatio != "16:9" && in.AspectRatio != "9:16" && in.AspectRatio != "1:1") {
		writeError(g, Invalid("invalid_reference_image", errors.New("请选择生图模型、素材类型、画幅并填写名称和提示词")))
		return
	}
	p, err := s.Projects.Get(ctx, g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	if !p.HasDraft(g.Param("draftId")) {
		writeError(g, Invalid("draft_not_found", errors.New("draft does not belong to project")))
		return
	}
	identity := sha256.Sum256([]byte(p.ID + "\x00" + g.Param("draftId") + "\x00" + in.IdempotencyKey))
	id := "reference-" + hex.EncodeToString(identity[:16])
	raw, _ := json.Marshal(in)
	hash := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(hash[:])
	if old, err := s.Store.Get(ctx, id); err == nil {
		if old.RequestHash != fingerprint {
			writeError(g, Conflict("idempotency_key_reused", errors.New("request parameters changed")))
			return
		}
		g.JSON(202, old)
		return
	} else if !errors.Is(err, fault.ErrNotFound) {
		writeError(g, err)
		return
	}
	if s.Providers.Catalog.Models == nil {
		writeError(g, errors.New("model repository unavailable"))
		return
	}
	model, err := s.Providers.Catalog.Models.GetModel(ctx, in.ModelID)
	if err != nil || !model.Supports(provider.Image) {
		writeError(g, Invalid("unsupported_model", errors.New("请选择可用的生图模型")))
		return
	}
	item := task.New(id, task.KindImage, task.Input{Prompt: in.Prompt, AspectRatio: in.AspectRatio, MaterialName: in.Name}, time.Now().UTC())
	item.ProjectID = p.ID
	item.DraftID = g.Param("draftId")
	item.ModelID = in.ModelID
	item.Target = "reference_" + in.Kind
	item.IdempotencyKey = in.IdempotencyKey
	item.RequestHash = fingerprint
	repo, ok := s.Store.(interface {
		CreateIfAbsent(context.Context, task.Task) (task.Task, bool, error)
	})
	if !ok {
		writeError(g, errors.New("atomic task repository required"))
		return
	}
	item, created, err := repo.CreateIfAbsent(ctx, item)
	if err != nil {
		writeError(g, err)
		return
	}
	if item.RequestHash != fingerprint {
		writeError(g, Conflict("idempotency_key_reused", errors.New("request parameters changed")))
		return
	}
	if created {
		s.broadcast(ctx, item)
		if s.Enqueuer != nil {
			if err = s.Enqueuer.Enqueue(ctx, item); err != nil {
				writeError(g, s.rejectSubmission(ctx, item, err))
				return
			}
		}
	}
	g.JSON(202, item)
}
