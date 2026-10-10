package task

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
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
	VideoInputMode       string            `json:"video_input_mode,omitempty"`
	FirstFrameMaterialID string            `json:"first_frame_material_id,omitempty"`
	LastFrameMaterialID  string            `json:"last_frame_material_id,omitempty"`
	ModelConfig          *provider.Model   `json:"model_config,omitempty"`
	DependencyHash       string            `json:"dependency_hash"`
	ID                   string            `json:"id"`
	SchemaVersion        int               `json:"schema_version"`
	ProjectID            string            `json:"project_id"`
	DraftID              string            `json:"draft_id"`
	SceneID              string            `json:"scene_id"`
	DraftVersion         int64             `json:"draft_version"`
	Entities             []SnapshotEntity  `json:"entities"`
	Utterances           []story.Utterance `json:"utterances"`
	Speech               []SpeechInput     `json:"speech,omitempty"`
	Prompt               string            `json:"prompt"`
	ModelID              string            `json:"model_id"`
	CapabilityVersion    string            `json:"capability_version"`
	Path                 string            `json:"path"`
	AspectRatio          string            `json:"aspect_ratio"`
	Resolution           string            `json:"resolution"`
	DurationSeconds      int               `json:"duration_seconds"`
	InputHash            string            `json:"input_hash"`
	CreatedAt            time.Time         `json:"created_at"`
}

func FreezeSnapshot(value GenerationSnapshot) (GenerationSnapshot, error) {
	if value.VideoInputMode != "" {
		if value.VideoInputMode != "first_frame" && value.VideoInputMode != "reference_images" {
			return value, errors.New("invalid video input mode")
		}
		if value.VideoInputMode == "reference_images" && (value.FirstFrameMaterialID != "" || value.LastFrameMaterialID != "") {
			return value, errors.New("reference mode cannot contain frame selections")
		}
		if value.VideoInputMode == "first_frame" {
			selected := map[string]bool{}
			for _, entity := range value.Entities {
				for _, ref := range entity.References {
					selected[ref.MaterialID] = true
				}
			}
			if value.FirstFrameMaterialID == "" || !selected[value.FirstFrameMaterialID] {
				return value, errors.New("snapshot first frame is missing")
			}
			if value.LastFrameMaterialID != "" && !selected[value.LastFrameMaterialID] {
				return value, errors.New("snapshot last frame is missing")
			}
		}
	}

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
	VideoModelConfig *provider.Model      `json:"video_model_config,omitempty"`
	CreatedAt        *time.Time           `json:"created_at,omitempty"`
	UpdatedAt        *time.Time           `json:"updated_at,omitempty"`
	Selection        *RunSelection        `json:"selection,omitempty"`
	Steps            []Task               `json:"steps,omitempty"`
	RequestHash      string               `json:"request_hash,omitempty"`
	VideoModelID     string               `json:"video_model_id,omitempty"`
	Resolution       string               `json:"resolution,omitempty"`
	DurationSeconds  int                  `json:"duration_seconds,omitempty"`
	Plan             []GenerationSnapshot `json:"plan,omitempty"`
	ID               string               `json:"id"`
	SceneID          string               `json:"scene_id"`
	State            string               `json:"state"`
	TaskIDs          []string             `json:"task_ids"`
	SnapshotID       string               `json:"snapshot_id"`
	Error            string               `json:"error,omitempty"`
}

type RunSelection struct {
	ImageTakeID    string            `json:"image_take_id,omitempty"`
	SpeechTakeIDs  map[string]string `json:"speech_take_ids,omitempty"`
	VideoVersionID string            `json:"video_version_id,omitempty"`
}
