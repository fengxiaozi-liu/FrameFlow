package project

import (
	"testing"
	"time"
)

func TestProjectDraft(t *testing.T) {
	p, e := New("p", "demo", time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if e = p.AddDraft(Draft{ID: "d"}); e != nil || len(p.Drafts) != 1 || p.Drafts[0].ProjectID != "p" {
		t.Fatal(e, p)
	}
}
