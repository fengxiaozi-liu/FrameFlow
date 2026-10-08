package sqlite

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"path/filepath"
	"testing"
	"time"
)

func TestEntityHistoryRetainsMaterialsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	p := filepath.Join(t.TempDir(), "test.db")
	r, err := Open(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.SaveMaterial(ctx, material.Asset{ID: "m", Name: "reference", Kind: material.Character, URL: "/media/m.png"}); err != nil {
		t.Fatal(err)
	}
	if err = r.SaveMaterial(ctx, material.Asset{ID: "v", Name: "voice", Kind: material.Voice, URL: "/media/v.wav"}); err != nil {
		t.Fatal(err)
	}
	value, _ := project.New("p", "test", time.Now())
	if err = value.AddEntity(project.EntityVersion{ID: "c", Name: "Ming", Kind: "character", Revision: 1, VoiceMaterialID: "v", References: []project.ReferenceImage{{MaterialID: "m", Order: 0}}}, 0); err != nil {
		t.Fatal(err)
	}
	if err = r.SaveProject(ctx, value); err != nil {
		t.Fatal(err)
	}
	if err = r.DeleteMaterial(ctx, "m"); err == nil {
		t.Fatal("referenced file could be deleted")
	}
	r.Close()
	r, err = Open(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := r.GetProject(ctx, "p")
	if err != nil || got.Version != 1 || len(got.Entities) != 1 {
		t.Fatal(got, err)
	}
	if got.Entities[0].VoiceMaterialID != "v" {
		t.Fatal("voice reference was lost")
	}
	if err = r.DeleteMaterial(ctx, "v"); err == nil {
		t.Fatal("referenced voice could be deleted after reopen")
	}
	repo := NewProjectRepository(r)
	if _, err = repo.Update(ctx, "p", func(v *project.Project) error {
		return v.AddEntity(project.EntityVersion{ID: "c", Name: "Ming", Kind: "character", Revision: 2, Archived: true, References: []project.ReferenceImage{}}, 1)
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.DeleteMaterial(ctx, "m"); err == nil {
		t.Fatal("archival lost historical material protection")
	}
	if err = r.DeleteMaterial(ctx, "v"); err == nil {
		t.Fatal("historical voice reference was lost")
	}
	if err = migrateEntities(ctx, r.db); err != nil {
		t.Fatal(err)
	}
}
