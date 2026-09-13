package task

import (
	"testing"
	"time"
)

func TestTaskLifecycle(t *testing.T) {
	x := New("1", "video", time.Now())
	if x.Status != StatusQueued {
		t.Fatal(x.Status)
	}
	x.Start(time.Now())
	x.Advance(130, "processing", time.Now())
	if x.Progress != 100 {
		t.Fatal(x.Progress)
	}
	x.Succeed(time.Now())
	if x.Status != StatusSucceeded {
		t.Fatal(x.Status)
	}
}
