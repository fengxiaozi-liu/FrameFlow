package task

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"time"
)

type SnapshotEntity struct {
	BindingID   string           `json:"binding_id"`
	EntityID    string           `json:"entity_id"`
	Revision    int64            `json:"revision"`
	Description string           `json:"description"`
	References  []MediaReference `json:"references"`
}
type MediaReference struct {
	MaterialID  string `json:"material_id"`
	ContentHash string `json:"content_hash"`
	Label       string `json:"label,omitempty"`
	URL         string `json:"url,omitempty"`
}
type SpeechInput struct {
	VoiceProfileID string `json:"voice_profile_id"`
	VoiceRevision  int64  `json:"voice_revision"`
	ModelID        string `json:"model_id"`
	Voice          string `json:"voice"`
	Language       string `json:"language"`
	Text           string `json:"text"`
	Instructions   string `json:"instructions,omitempty"`
}
type GenerationSnapshot struct {
	DependencyHash    string            `json:"dependency_hash"`
	ID                string            `json:"id"`
	SchemaVersion     int               `json:"schema_version"`
	ProjectID         string            `json:"project_id"`
	DraftID           string            `json:"draft_id"`
	SceneID           string            `json:"scene_id"`
	DraftVersion      int64             `json:"draft_version"`
	Entities          []SnapshotEntity  `json:"entities"`
	Utterances        []story.Utterance `json:"utterances"`
	Speech            []SpeechInput     `json:"speech,omitempty"`
	Prompt            string            `json:"prompt"`
	ModelID           string            `json:"model_id"`
	CapabilityVersion string            `json:"capability_version"`
	Path              string            `json:"path"`
	AspectRatio       string            `json:"aspect_ratio"`
	Resolution        string            `json:"resolution"`
	DurationSeconds   int               `json:"duration_seconds"`
	InputHash         string            `json:"input_hash"`
	CreatedAt         time.Time         `json:"created_at"`
}

func FreezeSnapshot(value GenerationSnapshot) (GenerationSnapshot, error) {
	if value.ProjectID == "" || value.DraftID == "" || value.SceneID == "" || value.ModelID == "" || value.CapabilityVersion == "" {
		return value, errors.New("incomplete generation snapshot")
	}
	b, err := json.Marshal(value)
	if err != nil {
		return value, err
	}
	var frozen GenerationSnapshot
	if err = json.Unmarshal(b, &frozen); err != nil {
		return value, err
	}
	canonical := frozen
	canonical.ID = ""
	canonical.InputHash = ""
	canonical.DraftVersion = 0
	canonical.CreatedAt = time.Time{}
	// Hash identity/content, never expiring delivery URLs.
	b, _ = json.Marshal(canonical)
	canonical = GenerationSnapshot{}
	_ = json.Unmarshal(b, &canonical)
	for i := range canonical.Entities {
		for j := range canonical.Entities[i].References {
			r := &canonical.Entities[i].References[j]
			if r.MaterialID == "" || r.ContentHash == "" {
				return value, errors.New("reference requires stable identity and digest")
			}
			r.URL = ""
		}
	}
	b, err = json.Marshal(canonical)
	if err != nil {
		return value, err
	}
	hash := sha256.Sum256(b)
	frozen.InputHash = hex.EncodeToString(hash[:])
	frozen.ID = "snapshot-" + frozen.InputHash
	return frozen, nil
}

type GenerationRun struct {
	RequestHash     string               `json:"request_hash,omitempty"`
	VideoModelID    string               `json:"video_model_id,omitempty"`
	Resolution      string               `json:"resolution,omitempty"`
	DurationSeconds int                  `json:"duration_seconds,omitempty"`
	Plan            []GenerationSnapshot `json:"plan,omitempty"`
	ID              string               `json:"id"`
	SceneID         string               `json:"scene_id"`
	State           string               `json:"state"`
	TaskIDs         []string             `json:"task_ids"`
	SnapshotID      string               `json:"snapshot_id"`
	Error           string               `json:"error,omitempty"`
}
