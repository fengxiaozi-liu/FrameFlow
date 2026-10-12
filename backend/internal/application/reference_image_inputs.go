package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type ImageReferenceReader interface {
	SnapshotReference(context.Context, material.Asset, string, int64) (task.MediaReference, error)
}

// Resolve stable IDs in user order; local project images never become library assets here.
func (s TaskService) resolveImageReferences(ctx context.Context, projectID string, ids []string, maxBytes int64) ([]task.MediaReference, error) {
	refs := make([]task.MediaReference, 0, len(ids))
	seen := map[string]bool{}
	for i, id := range ids {
		if id == "" || seen[id] {
			return nil, errors.New("参考图片不能为空或重复")
		}
		seen[id] = true
		if s.Materials == nil {
			return nil, errors.New("素材仓储不可用")
		}
		asset, err := s.Materials.Get(ctx, id)
		if err != nil {
			return nil, errors.New("参考图片不存在，请重新选择")
		}
		if asset.ReferenceOnly && asset.ProjectID != projectID {
			return nil, errors.New("不能使用其他项目的私有参考图片")
		}
		if asset.Kind != material.Character && asset.Kind != material.Scene && asset.Kind != material.Prop && asset.Kind != material.Visual && asset.Kind != material.Frame {
			return nil, errors.New("请选择图片素材")
		}
		if s.ImageReferences == nil {
			return nil, errors.New("图片读取服务未配置")
		}
		r, err := s.ImageReferences.SnapshotReference(ctx, asset, fmt.Sprintf("图%d", i+1), maxBytes)
		if err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, nil
}
