package story

import (
	"strings"
	"testing"
)

func TestEntityGenerationResolvesKeysAndRejectsForeignIdentity(t *testing.T) {
	raw := `{"entities":[{"key":"c1","kind":"character","name":"小明","description":"蓝外套"},{"key":"c2","kind":"character","name":"小明","description":"黑外套"}],"scenes":[{"title":"交谈","visual_prompt":"两人交谈","narration":"","duration_seconds":4,"entity_keys":["c1","c2"],"dialogue":[{"speaker_key":"c2","text":"你好"}]}]}`
	c, err := ParseCandidateResult(CandidateStoryboard, raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.Entities[0].ID == c.Entities[1].ID || c.Scenes[0].Utterances[0].SpeakerID != c.Entities[1].ID {
		t.Fatal("identity mismatch")
	}
	if _, err = ParseCandidateResult(CandidateStoryboard, strings.Replace(raw, `"speaker_key":"c2"`, `"speaker_key":"unknown"`, 1)); err == nil {
		t.Fatal("unknown speaker accepted")
	}
	if _, err = ParseCandidateResult(CandidateStoryboard, strings.Replace(raw, `"key":"c2"`, `"key":"c1"`, 1)); err == nil {
		t.Fatal("duplicate key accepted")
	}
}
