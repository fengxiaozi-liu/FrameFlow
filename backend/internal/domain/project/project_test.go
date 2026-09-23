package project

import (
	"encoding/json"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"testing"
	"time"
)

func TestEmptyStoryboardSnapshotSerializesScenesAsArray(t *testing.T) {
	d := Draft{ID: "d"}
	if err := d.CaptureStoryboardSnapshot("before-first-storyboard", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(d.StoryboardSnapshots[0])
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Scenes []story.Scene `json:"scenes"`
	}
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Scenes == nil {
		t.Fatalf("empty snapshot scenes must be a JSON array: %s", encoded)
	}
}

func TestDraftCandidatesSnapshotsAndSceneScopedMedia(t *testing.T) {
	now := time.Now().UTC()
	d := Draft{ID: "d", Story: story.Document{Scenes: []story.Scene{{ID: "s1", Order: 1, Title: "one"}, {ID: "s2", Order: 2, Title: "two"}}}}
	if err := d.AddCandidate(story.Candidate{ID: "c1", Target: story.CandidateStoryboard, Status: "preview"}); err != nil {
		t.Fatal(err)
	}
	if len(d.Story.Scenes) != 2 || len(d.Candidates) != 1 {
		t.Fatal("candidate must not change active scenes")
	}
	if err := d.AddBinding(SceneBinding{SceneID: "s1", MaterialID: "m1", Usage: "first_frame"}); err != nil {
		t.Fatal(err)
	}
	if err := d.AddBinding(SceneBinding{SceneID: "other", MaterialID: "m1", Usage: "first_frame"}); err == nil {
		t.Fatal("cross-scene binding accepted")
	}
	if err := d.AddVideoVersion(VideoVersion{ID: "v1", SceneID: "s1", TaskID: "t1", LocalMediaPath: "clip.mp4"}); err != nil {
		t.Fatal(err)
	}
	if err := d.SelectVideoVersion("s2", "v1"); err == nil {
		t.Fatal("version selected for wrong scene")
	}
	if err := d.SelectVideoVersion("s1", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := d.CaptureStoryboardSnapshot("snap1", now); err != nil {
		t.Fatal(err)
	}
	copied, err := d.CopyScene("s1")
	if err != nil || copied.ID == "s1" || len(d.Bindings) != 2 || len(d.VideoVersions) != 1 {
		t.Fatalf("copy must inherit only bindings: %+v, %v", d, err)
	}
	d.SelectedVersions["s1"] = "changed"
	if d.StoryboardSnapshots[0].SelectedVersions["s1"] != "v1" {
		t.Fatal("snapshot aliased active selection")
	}
	if err := d.AddComposition(Composition{ID: "film", TaskID: "compose", ClipVersionIDs: []string{"v1", "v1", "v1"}}); err == nil {
		t.Fatal("composition accepted version from wrong scene")
	}
}

func TestUpdateDraftBodyRejectsStaleRevision(t *testing.T) {
	now := time.Now().UTC()
	p, _ := New("p", "demo", now)
	if err := p.AddDraft(Draft{ID: "d"}); err != nil {
		t.Fatal(err)
	}
	first, err := p.UpdateDraftBody("d", 0, "first", now)
	if err != nil || first.Version != 1 {
		t.Fatalf("first save: %+v, %v", first, err)
	}
	latest, err := p.UpdateDraftBody("d", 0, "stale", now)
	if !errors.Is(err, ErrVersionConflict) || latest.Story.Body != "first" {
		t.Fatalf("stale editor overwrote body: %+v, %v", latest, err)
	}
	empty, err := p.UpdateDraftBody("d", 1, "", now)
	if err != nil || empty.Story.Body != "" || empty.Version != 2 {
		t.Fatalf("clearing editor failed: %+v, %v", empty, err)
	}
}

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
