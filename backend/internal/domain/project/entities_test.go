package project

import "testing"

func TestEntityVersionIsolation(t *testing.T) {
	c := EntityCatalog{}
	e := EntityVersion{ID: "a", Kind: "character", Name: "Ming", Revision: 1, References: []ReferenceImage{{MaterialID: "m", Order: 0}}}
	if err := c.AddEntity(e, 0); err != nil {
		t.Fatal(err)
	}
	e.References[0].MaterialID = "changed"
	saved, _ := c.Entity("a", 1)
	if saved.References[0].MaterialID != "m" {
		t.Fatal("caller mutated saved revision")
	}
	saved.References[0].MaterialID = "also changed"
	saved, _ = c.Entity("a", 1)
	if saved.References[0].MaterialID != "m" {
		t.Fatal("read leaked mutable revision")
	}
	e.Revision = 2
	if err := c.AddEntity(e, 0); err != ErrCatalogConflict {
		t.Fatal("stale catalog accepted")
	}
	e.Kind = "location"
	if err := c.AddEntity(e, 1); err == nil {
		t.Fatal("identity type changed")
	}
}
