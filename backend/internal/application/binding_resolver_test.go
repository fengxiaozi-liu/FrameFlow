package application

import (
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"testing"
)

func TestResolveScopesReferencesAndVoice(t *testing.T) {
	c := project.EntityCatalog{Entities: []project.EntityVersion{{ID: "c", Kind: "character", Name: "Ming", Revision: 1, Description: "blue jacket", VoiceProfileID: "v", VoiceProfileRevision: 1, References: []project.ReferenceImage{{MaterialID: "a"}, {MaterialID: "b"}}}}, Voices: []project.VoiceProfile{{ID: "v", Revision: 1, RemoteVoiceID: "Ethan"}}}
	ids := []string{"b", "a"}
	s := story.Scene{EntityBindings: []story.EntityBinding{{ID: "b", EntityID: "c", EntityRevision: 1, Role: "cast", DescriptionSupplement: "smiling", SelectedMaterialIDs: &ids}}, PromptDocument: &story.PromptDocument{SchemaVersion: 1, Nodes: []story.PromptNode{{Type: "entity_ref", BindingID: "b"}}}, Utterances: []story.Utterance{{ID: "u", SpeakerType: "character", SpeakerID: "c", SpeakerRevision: 1, Text: "hi"}}}
	r, err := ResolveBindings(c, s, true)
	if err != nil || r.Prompt != "Ming" || r.Entities[0].References[0].MaterialID != "b" || r.Voices["u"].RemoteVoiceID != "Ethan" {
		t.Fatal(r, err)
	}
	ids[0] = "unrelated"
	if _, err = ResolveBindings(c, s, false); err == nil {
		t.Fatal("unrelated reference accepted")
	}
}
