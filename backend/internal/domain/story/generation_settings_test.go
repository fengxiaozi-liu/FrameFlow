package story

import (
	"encoding/json"
	"testing"
)

func TestGenerationSettingsCompatibility(t *testing.T) {
	var old Scene
	if err := json.Unmarshal([]byte(`{"id":"s","duration_seconds":6}`), &old); err != nil || old.GenerationSettings != nil {
		t.Fatal("old scene incompatible", err)
	}
	s := GenerationSettings{ImageModelID: "image", VideoModelID: "video", AspectRatio: "16:9", Resolution: "720P", DurationSeconds: 6}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	old.GenerationSettings = &s
	b, _ := json.Marshal(old)
	var next Scene
	if err := json.Unmarshal(b, &next); err != nil || *next.GenerationSettings != s {
		t.Fatal("settings round trip failed", err)
	}
	s.DurationSeconds = 0
	if s.Validate() == nil {
		t.Fatal("invalid duration accepted")
	}
}
