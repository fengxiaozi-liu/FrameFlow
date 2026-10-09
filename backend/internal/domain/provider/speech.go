package provider

import "context"

const Speech Capability = "speech"

type ImageReference struct {
	URL      string `json:"url"`
	EntityID string `json:"entity_id"`
	Label    string `json:"label"`
}
type SpeechRequest struct{ Text, Voice, Language, Instructions string }
type SpeechResult struct{ URL string }
type Voice struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Languages []string `json:"languages"`
}
type SpeechClient interface {
	GenerateSpeech(context.Context, Connection, Model, SpeechRequest, RequestOptions) (SpeechResult, error)
	Voices(Model) []Voice
}
type InputCapability struct {
	Operations      map[Capability]OperationStatus `json:"operations"`
	Ratios          []string                       `json:"ratios"`
	ImageRatios     []string                       `json:"image_ratios"`
	Version         string                         `json:"version"`
	ModelID         string                         `json:"model_id"`
	ReferenceImages int                            `json:"reference_images"`
	Speech          bool                           `json:"speech"`
	ReferenceVideo  bool                           `json:"reference_video"`
	FirstFrame      bool                           `json:"first_frame"`
	Durations       []int                          `json:"durations"`
	Resolutions     []string                       `json:"resolutions"`
	Voices          []Voice                        `json:"voices"`
}
