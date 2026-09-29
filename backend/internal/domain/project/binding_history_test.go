package project

import (
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"testing"
	"time"
)

func TestBindingCopyAndSnapshotOwnTheirData(t *testing.T) {
	d := Draft{ID: "d", Story: story.Document{Scenes: []story.Scene{{ID: "s", Title: "shot", EntityBindings: []story.EntityBinding{{ID: "b", EntityID: "c", EntityRevision: 1, Role: "cast"}}, PromptDocument: &story.PromptDocument{SchemaVersion: 1, Nodes: []story.PromptNode{{Type: "entity_ref", BindingID: "b"}}}, Utterances: []story.Utterance{{ID: "u", SpeakerType: "character", SpeakerID: "c", SpeakerRevision: 1, Text: "hi"}}}}}, MediaTakes: []MediaTake{{ID: "take"}}, SelectedMedia: map[string]string{"s:speech:u": "take"}}
	if err := d.CaptureStoryboardSnapshot("snap", time.Now()); err != nil {
		t.Fatal(err)
	}
	copied, err := d.CopyScene("s")
	if err != nil {
		t.Fatal(err)
	}
	if copied.EntityBindings[0].ID == "b" || copied.PromptDocument.Nodes[0].BindingID != copied.EntityBindings[0].ID || copied.Utterances[0].ID == "u" {
		t.Fatal("local identities not remapped")
	}
	d.Story.Scenes[1].Utterances[0].Text = "changed"
	d.Story.Scenes[0].EntityBindings[0].DescriptionSupplement = "new"
	d.SelectedMedia["s:speech:u"] = "other"
	snap := d.StoryboardSnapshots[0]
	if snap.Scenes[0].EntityBindings[0].DescriptionSupplement != "" || snap.SelectedMedia["s:speech:u"] != "take" || d.Story.Scenes[0].Utterances[0].Text != "hi" {
		t.Fatal("shared mutable history")
	}
}
