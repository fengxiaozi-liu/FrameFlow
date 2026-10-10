package project

import (
	"errors"
	"testing"
	"time"
)

func TestUpdateDraftContentTitleAndConflict(t *testing.T) {
	p := Project{Name: "未命名视频", Drafts: []Draft{{ID: "a", Name: "未命名视频", Version: 1}}}
	name := "测试视频"
	saved, err := p.UpdateDraftContent("a", 1, "故事", &name, time.Now())
	if err != nil || p.Name != name || saved.Name != name || saved.Version != 2 {
		t.Fatalf("rename: %+v %+v %v", p, saved, err)
	}
	stale := "旧标题"
	if _, err = p.UpdateDraftContent("a", 1, "old", &stale, time.Now()); !errors.Is(err, ErrVersionConflict) || p.Name != name {
		t.Fatalf("stale rename changed project: %+v %v", p, err)
	}
	blank := " "
	if _, err = p.UpdateDraftContent("a", 2, "changed", &blank, time.Now()); err == nil || p.Drafts[0].Version != 2 || p.Drafts[0].Story.Body != "故事" {
		t.Fatal("invalid title mutated draft")
	}
	p.Drafts = append(p.Drafts, Draft{ID: "b", Name: "另一个草稿"})
	if _, err = p.UpdateDraftContent("a", 2, "故事", &stale, time.Now()); err != nil || p.Name != name || p.Drafts[1].Name != "另一个草稿" {
		t.Fatalf("multi-draft rename: %+v %v", p, err)
	}
}

func TestDeletionRequiresWholeProjectSnapshot(t *testing.T) {
	p := Project{Name: "项目", EntityCatalog: EntityCatalog{Version: 2}, Drafts: []Draft{{ID: "a", Version: 1}, {ID: "b", Version: 3}}}
	correct := DeletionExpectation{Name: "项目", CatalogVersion: 2, DraftVersions: map[string]int64{"a": 1, "b": 3}}
	if err := p.CheckDeletion(correct); err != nil {
		t.Fatal(err)
	}
	for _, wrong := range []DeletionExpectation{
		{Name: "旧名称", CatalogVersion: 2, DraftVersions: correct.DraftVersions},
		{Name: "项目", CatalogVersion: 1, DraftVersions: correct.DraftVersions},
		{Name: "项目", CatalogVersion: 2, DraftVersions: map[string]int64{"a": 1}},
		{Name: "项目", CatalogVersion: 2, DraftVersions: map[string]int64{"a": 1, "c": 3}},
		{Name: "项目", CatalogVersion: 2, DraftVersions: map[string]int64{"a": 1, "b": 2}},
	} {
		if p.CheckDeletion(wrong) == nil {
			t.Fatalf("accepted stale snapshot: %+v", wrong)
		}
	}
}
