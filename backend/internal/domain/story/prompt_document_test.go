package story

import "testing"

func TestPromptUsesIdentityAndRejectsDanglingReference(t *testing.T) {
	d := PromptDocument{SchemaVersion: 1, Nodes: []PromptNode{{Type: "text", Text: "你好 "}, {Type: "entity_ref", BindingID: "b1"}, {Type: "text", Text: " / "}, {Type: "entity_ref", BindingID: "b2"}}}
	got, err := d.Render(map[string]string{"b1": "同名", "b2": "新名字"})
	if err != nil || got != "你好 同名 / 新名字" {
		t.Fatal(got, err)
	}
	if _, err = d.Render(map[string]string{"b1": "同名"}); err == nil {
		t.Fatal("dangling reference allowed")
	}
	if err = ValidateUtterances([]Utterance{{ID: "u", SpeakerType: "character", SpeakerID: "offscreen", SpeakerRevision: 1, Text: "hello", Order: 0}}, map[string]bool{"offscreen": true}); err != nil {
		t.Fatal(err)
	}
	if err = ValidateUtterances([]Utterance{{ID: "u", SpeakerType: "narrator", Text: "hello"}}, nil); err != nil {
		t.Fatal(err)
	}
}
