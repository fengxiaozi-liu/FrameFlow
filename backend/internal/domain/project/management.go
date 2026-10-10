package project

import (
	"errors"
	"strings"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
)

// DeletionExpectation describes the project the user reviewed before confirming.
type DeletionExpectation struct {
	Name           string           `json:"expected_name"`
	CatalogVersion int64            `json:"expected_catalog_version"`
	DraftVersions  map[string]int64 `json:"expected_draft_versions"`
}

func (p Project) CheckDeletion(expected DeletionExpectation) error {
	matches := p.Name == expected.Name && p.EntityCatalog.Version == expected.CatalogVersion && len(p.Drafts) == len(expected.DraftVersions)
	for _, d := range p.Drafts {
		version, exists := expected.DraftVersions[d.ID]
		matches = matches && exists && version == d.Version
	}
	if !matches {
		return &fault.Error{Kind: "conflict", Code: "project_version_conflict", Cause: errors.New("项目已在其他页面更新，请刷新后确认再删除")}
	}
	return nil
}

// A single-draft creation has one user-facing title. Multiple drafts keep their
// own titles without renaming the containing project or sibling drafts.
func (p *Project) UpdateDraftContent(id string, version int64, body string, name *string, now time.Time) (Draft, error) {
	if name != nil && strings.TrimSpace(*name) == "" {
		return Draft{}, &fault.Error{Kind: "invalid", Code: "invalid_draft_name", Cause: errors.New("名称不能为空")}
	}
	saved, err := p.UpdateDraftBody(id, version, body, now)
	if err != nil || name == nil {
		return saved, err
	}
	for i := range p.Drafts {
		if p.Drafts[i].ID == id {
			p.Drafts[i].Name = strings.TrimSpace(*name)
			saved = p.Drafts[i]
			if len(p.Drafts) == 1 {
				p.Name = saved.Name
			}
			break
		}
	}
	return saved, nil
}
