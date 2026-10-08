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
	Version         string   `json:"version"`
	ModelID         string   `json:"model_id"`
	ReferenceImages int      `json:"reference_images"`
	Speech          bool     `json:"speech"`
	ReferenceVideo  bool     `json:"reference_video"`
	FirstFrame      bool     `json:"first_frame"`
	Durations       []int    `json:"durations"`
	Resolutions     []string `json:"resolutions"`
	Voices          []Voice  `json:"voices"`
}

func InputCapabilityFor(id string) (InputCapability, bool) {
	c := InputCapability{Version: "bindings.v1", ModelID: id, Voices: []Voice{}, Durations: []int{}, Resolutions: []string{}}
	switch id {
	case "qwen-image-2.0-pro":
		c.ReferenceImages = 3
	case "qwen3-tts-flash":
		c.Speech = true
		c.Voices = []Voice{{ID: "Cherry", Name: "Cherry", Languages: []string{"Chinese", "English"}}, {ID: "Ethan", Name: "Ethan", Languages: []string{"Chinese", "English"}}}
	case "wan2.7-i2v", "wan2.7-i2v-2026-04-25":
		c.FirstFrame = true
		c.Resolutions = []string{"720P", "1080P"}
		for i := 2; i <= 15; i++ {
			c.Durations = append(c.Durations, i)
		}
	case "wan2.7-r2v", "wan2.7-r2v-2026-06-12":
		c.Version = "bindings.r2v.v1"
		c.ReferenceVideo = true
		c.ReferenceImages = 5
		c.Resolutions = []string{"720P", "1080P"}
		for i := 2; i <= 15; i++ {
			c.Durations = append(c.Durations, i)
		}
	default:
		return c, false
	}
	return c, true
}
