package task

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestRunSelectionRoundTrip(t *testing.T) {
	var legacy GenerationRun
	if err := json.Unmarshal([]byte(`{"id":"old","task_ids":[]}`), &legacy); err != nil || legacy.Selection != nil {
		t.Fatal("legacy incompatible", err)
	}
	oldJSON, _ := json.Marshal(legacy)
	if bytes.Contains(oldJSON, []byte(`"created_at"`)) {
		t.Fatal("legacy date was invented", string(oldJSON))
	}
	now := time.Now().UTC()
	run := GenerationRun{ID: "run", CreatedAt: &now, Selection: &RunSelection{ImageTakeID: "image", SpeechTakeIDs: map[string]string{"u": "speech"}, VideoVersionID: "version"}}
	b, _ := json.Marshal(run)
	var next GenerationRun
	if err := json.Unmarshal(b, &next); err != nil || next.Selection.ImageTakeID != "image" || next.Selection.SpeechTakeIDs["u"] != "speech" {
		t.Fatal("selection round trip", err)
	}
}
