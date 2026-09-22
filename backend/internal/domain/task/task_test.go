package task

import (
	"testing"
	"time"
)

func TestTaskLifecycle(t *testing.T) {
	x := New("1", KindVideo, Input{Prompt: "测试视频"}, time.Now())
	if x.Status != StatusQueued {
		t.Fatal(x.Status)
	}
	x.Start(time.Now())
	x.Advance(130, StageProcessing, time.Now())
	if x.Progress != 100 {
		t.Fatal(x.Progress)
	}
	x.Succeed(time.Now())
	if x.Status != StatusSucceeded {
		t.Fatal(x.Status)
	}
}

func TestInputRequiresPrompt(t *testing.T) {
	if err := (Input{}).Validate(); err == nil {
		t.Fatal("empty prompt should be rejected")
	}
}

func TestTaskScopeRequiresProjectAndDraft(t *testing.T) {
	item := New("1", KindStory, Input{Prompt: "test"}, time.Now())
	if err := item.ValidateScope(); err == nil {
		t.Fatal("missing scope should be rejected")
	}
	item.ProjectID, item.DraftID = "project", "draft"
	if err := item.ValidateScope(); err != nil {
		t.Fatal(err)
	}
}
