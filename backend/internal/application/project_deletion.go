package application

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
)

type ProjectDeletion struct {
	Repository project.ProjectDeletionRepository
}

func (s ProjectDeletion) Delete(ctx context.Context, id string, expected project.DeletionExpectation) error {
	if expected.DraftVersions == nil || expected.CatalogVersion < 0 {
		return Invalid("expected_project_required", errors.New("请提供项目及所有草稿的当前版本"))
	}
	for id, version := range expected.DraftVersions {
		if id == "" || version < 0 {
			return Invalid("invalid_draft_version", errors.New("草稿版本无效"))
		}
	}
	if s.Repository == nil {
		return unavailable("project_delete_unavailable", "项目删除暂不可用")
	}
	return s.Repository.DeleteProjectChecked(ctx, id, expected)
}
