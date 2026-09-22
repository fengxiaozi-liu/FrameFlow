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

func TestApplyTaskResultTargetsDraftAndIsIdempotent(t *testing.T) {
	now := time.Now().UTC()
	p, _ := New("p", "demo", now)
	_ = p.AddDraft(Draft{ID: "first"})
	_ = p.AddDraft(Draft{ID: "second"})
	result := DraftOutput{TaskID: "task-1", Kind: "story", Text: "generated story"}
	if err := p.ApplyTaskResult("second", result, now); err != nil {
		t.Fatal(err)
	}
	if p.Drafts[0].Story.Body != "" || p.Drafts[1].Story.Body != "generated story" {
		t.Fatalf("result applied to wrong draft: %#v", p.Drafts)
	}
	if err := p.ApplyTaskResult("second", result, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(p.Drafts[1].Outputs) != 1 {
		t.Fatalf("duplicate result was appended: %#v", p.Drafts[1].Outputs)
	}
	if err := p.ApplyTaskResult("missing", result, now); err == nil {
		t.Fatal("missing draft was accepted")
	}
}
