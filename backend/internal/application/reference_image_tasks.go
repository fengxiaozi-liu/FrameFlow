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
	"strings"
	"time"
	"unicode/utf8"
)

type ReferenceImageRequest struct {
	Mode                 string   `json:"mode,omitempty"`
	ReferenceMaterialIDs []string `json:"reference_material_ids,omitempty"`
	IdempotencyKey       string   `json:"idempotency_key"`
	Kind                 string   `json:"kind"`
	Name                 string   `json:"name"`
	Prompt               string   `json:"prompt"`
	ModelID              string   `json:"model_id"`
	AspectRatio          string   `json:"aspect_ratio"`
}

func (s TaskService) SubmitReferenceImage(ctx context.Context, projectID, draftID string, in ReferenceImageRequest) (task.Task, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Prompt = strings.TrimSpace(in.Prompt)
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > 128 || in.ModelID == "" || in.Name == "" || utf8.RuneCountInString(in.Name) > 100 || in.Prompt == "" || utf8.RuneCountInString(in.Prompt) > 4000 || (in.Kind != "scene" && in.Kind != "character") || provider.ValidateRatioValue(in.AspectRatio) != nil {
		return task.Task{}, Invalid("invalid_reference_image", errors.New("请选择生图模型、素材类型、画幅并填写名称和提示词"))
	}
	p, err := s.Projects.Get(ctx, projectID)
	if err != nil {
		return task.Task{}, err
	}
	if !p.HasDraft(draftID) {
		return task.Task{}, Invalid("draft_not_found", errors.New("draft does not belong to project"))
	}
	identity := sha256.Sum256([]byte(p.ID + "\x00" + draftID + "\x00" + in.IdempotencyKey))
	id := "reference-" + hex.EncodeToString(identity[:16])
	raw, _ := json.Marshal(in)
	hash := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(hash[:])
	if old, err := s.Store.Get(ctx, id); err == nil {
		if old.RequestHash != fingerprint {
			return task.Task{}, Conflict("idempotency_key_reused", errors.New("request parameters changed"))
		}
		return old, nil
	} else if !errors.Is(err, fault.ErrNotFound) {
		return task.Task{}, err
	}
	if s.Providers.Catalog.Models == nil {
		return task.Task{}, errors.New("model repository unavailable")
	}
	model, err := s.Providers.Catalog.Models.GetModel(ctx, in.ModelID)
	if err != nil || !model.Supports(provider.Image) {
		return task.Task{}, Invalid("unsupported_model", errors.New("请选择可用的生图模型"))
	}
	if err := model.Operations[provider.Image].ValidateAspectRatio(in.AspectRatio); err != nil {
		return task.Task{}, Invalid("invalid_ratio", err)
	}
	if in.Mode == "" {
		in.Mode = provider.TextToImage
	}
	binding := model.Resolve(provider.Image).Binding
	if err := binding.ValidateImageInput(in.Mode, len(in.ReferenceMaterialIDs)); err != nil {
		return task.Task{}, Invalid("invalid_image_mode", err)
	}
	references, err := s.resolveImageReferences(ctx, p.ID, in.ReferenceMaterialIDs, binding.MaxImageBytes)
	if err != nil {
		return task.Task{}, Invalid("invalid_image_references", err)
	}
	item := task.New(id, task.KindImage, task.Input{Mode: in.Mode, ReferenceImages: references, Prompt: in.Prompt, AspectRatio: in.AspectRatio, MaterialName: in.Name}, time.Now().UTC())
	item.ProjectID = p.ID
	item.DraftID = draftID
	item.ModelID = in.ModelID
	item.Target = "reference_" + in.Kind
	item.IdempotencyKey = in.IdempotencyKey
	item.RequestHash = fingerprint
	if err := freezeResolvedTaskModel(ctx, model, &item, s.Providers.Catalog.Connections); err != nil {
		return task.Task{}, Invalid("model_configuration", err)
	}
	repo, ok := s.Store.(interface {
		CreateIfAbsent(context.Context, task.Task) (task.Task, bool, error)
	})
	if !ok {
		return task.Task{}, errors.New("atomic task repository required")
	}
	item, created, err := repo.CreateIfAbsent(ctx, item)
	if err != nil {
		return task.Task{}, err
	}
	if item.RequestHash != fingerprint {
		return task.Task{}, Conflict("idempotency_key_reused", errors.New("request parameters changed"))
	}
	if created {
		s.broadcast(ctx, item)
		if s.Enqueuer != nil {
			if err = s.Enqueuer.Enqueue(ctx, item); err != nil {
				return task.Task{}, s.rejectSubmission(ctx, item, err)
			}
		}
	}
	return item, nil
}
