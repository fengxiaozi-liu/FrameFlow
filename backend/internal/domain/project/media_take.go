package project

type MediaTake struct {
	DependencyHash  string  `json:"dependency_hash"`
	ID              string  `json:"id"`
	Kind            string  `json:"kind"`
	SceneID         string  `json:"scene_id"`
	UtteranceID     string  `json:"utterance_id,omitempty"`
	TaskID          string  `json:"task_id"`
	MaterialID      string  `json:"material_id"`
	SnapshotID      string  `json:"snapshot_id"`
	InputHash       string  `json:"input_hash"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	Stale           bool    `json:"stale,omitempty"`
}
