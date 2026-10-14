package project

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"testing"
	"time"
)

func TestRemoveFailedRunPreservesMediaAndOtherHistory(t *testing.T) {
	p := Project{ID: "p", Drafts: []Draft{{ID: "d", Version: 7, GenerationRuns: []task.GenerationRun{{ID: "r", SceneID: "s", State: "failed"}, {ID: "other", SceneID: "s", State: "completed"}}, VideoVersions: []VideoVersion{{ID: "v", TaskID: "r-0"}}, SelectedVersions: map[string]string{"s": "v"}}}}
	now := time.Now().UTC()
	if err := p.RemoveFailedRun("d", "s", "r", []task.Task{{Status: task.StatusSucceeded}, {Status: task.StatusFailed}}, now); err != nil {
		t.Fatal(err)
	}
	d := p.Drafts[0]
	if len(d.GenerationRuns) != 1 || d.GenerationRuns[0].ID != "other" || d.Version != 8 || !d.UpdatedAt.Equal(now) || len(d.VideoVersions) != 1 || d.SelectedVersions["s"] != "v" {
		t.Fatalf("unexpected deletion: %+v", d)
	}
}

func TestRemoveFailedRunRejectsWrongScopeAndActiveSteps(t *testing.T) {
	for _, c := range []struct {
		state, scene string
		child        task.Status
		notFound     bool
	}{{"generating", "s", task.StatusRunning, false}, {"completed", "s", task.StatusSucceeded, false}, {"failed", "s", task.StatusQueued, false}, {"failed", "s", task.StatusRunning, false}, {"failed", "other", task.StatusFailed, true}} {
		p := Project{Drafts: []Draft{{ID: "d", Version: 4, GenerationRuns: []task.GenerationRun{{ID: "r", SceneID: "s", State: c.state}}}}}
		err := p.RemoveFailedRun("d", c.scene, "r", []task.Task{{Status: c.child}}, time.Now())
		if err == nil || len(p.Drafts[0].GenerationRuns) != 1 || p.Drafts[0].Version != 4 {
			t.Fatalf("mutation on rejection: %+v", c)
		}
		if c.notFound && !errors.Is(err, fault.ErrNotFound) {
			t.Fatal(err)
		}
	}
}
