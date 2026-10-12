package provider

import "strings"

// MigrateLegacyModel is deterministic and idempotent. Called on legacy persistence
// reads / catalogue creation, never by execution or resolution. Unknown IDs remain
// unconfigured; an explicit empty Operations map must not receive a preset again.
func MigrateLegacyModel(m Model) Model {
	if m.Operations != nil {
		return m
	}
	m.Operations = map[Capability]OperationBinding{}
	m.Revision = 1
	for _, op := range m.Capabilities {
		b := OperationBinding{Source: "legacy-preset.v1"}
		id := m.RemoteID
		switch op {
		case Story:
			if !(strings.HasPrefix(id, "qwen-") && !strings.Contains(id, "image") && !strings.Contains(id, "audio") && !strings.Contains(id, "vl") && !strings.Contains(id, "omni") || strings.HasPrefix(id, "deepseek-") || strings.HasPrefix(id, "kimi-") || strings.HasPrefix(id, "glm-")) {
				continue
			}
			b.Protocol = ChatProtocol
		case Image:
			b.Protocol = ImageProtocol
			b.Ratios = []string{"16:9", "9:16", "1:1"}
			switch {
			case strings.HasPrefix(id, "qwen-image-2.0"):
				b.ImageSizes = map[string]string{"16:9": "2688*1536", "9:16": "1536*2688", "1:1": "2048*2048"}
			case strings.HasPrefix(id, "qwen-image-max"), strings.HasPrefix(id, "qwen-image-plus"), id == "qwen-image":
				b.ImageSizes = map[string]string{"16:9": "1664*928", "9:16": "928*1664", "1:1": "1328*1328"}
			default:
				continue
			}
			if id == "qwen-image-2.0-pro" {
				b.MaxReferences = 3
				b.ImageModes = []string{TextToImage, ImageToImage}
				b.MinReferences = 1
				b.MaxImageBytes = 10 << 20
			}
		case Video:
			v, ok := legacyVideoPreset(id)
			if !ok {
				continue
			}
			b.Protocol = VideoProtocol
			b.InputMode = "first_frame"
			b.MaxReferences = 1
			if v.InputMode == "reference_images" {
				b.InputMode = "reference_images"
				b.MaxReferences = 5
			}
			b.Ratios = []string{"16:9", "9:16", "1:1"}
			b.Resolutions = v.Resolutions
			for i := v.MinDuration; i <= v.MaxDuration; i++ {
				b.Durations = append(b.Durations, i)
			}
			b.MaxImageBytes = v.MaxImageBytes
			b.LastFrame = v.AllowedMedia["last_frame"] > 0
			b.DrivingAudio = v.AllowedMedia["driving_audio"] > 0
			b.MaxAudioBytes = v.MaxAudioBytes
			b.MinAudioDuration = v.MinAudioDuration
			b.MaxAudioDuration = v.MaxAudioDuration
		case Speech:
			c, ok := legacyInputPreset(id)
			if !ok || !c.Speech {
				continue
			}
			b.Protocol = SpeechProtocol
			b.Voices = c.Voices
			b.MaxTextLength = 600
		default:
			continue
		}
		m.Operations[op] = b
	}
	return m
}

func legacyInputPreset(id string) (InputCapability, bool) {
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

func legacyVideoPreset(remoteModelID string) (VideoCapability, bool) {
	if remoteModelID == "wan2.7-r2v" || remoteModelID == "wan2.7-r2v-2026-06-12" {
		return VideoCapability{InputMode: "reference_images", ModelPrefix: "wan2.7-r2v", MinDuration: 2, MaxDuration: 15, Resolutions: []string{"720P", "1080P"}, AllowedMedia: map[string]int{"reference_image": 5}, MaxImageBytes: 10 << 20}, true
	}
	if remoteModelID != "wan2.7-i2v" && remoteModelID != "wan2.7-i2v-2026-04-25" {
		return VideoCapability{}, false
	}
	return VideoCapability{
		ModelPrefix:      "wan2.7-i2v",
		MinDuration:      2,
		MaxDuration:      15,
		Resolutions:      []string{"720P", "1080P"},
		AllowedMedia:     map[string]int{"first_frame": 1, "last_frame": 1, "driving_audio": 1},
		MaxImageBytes:    20 << 20,
		MaxAudioBytes:    15 << 20,
		MinAudioDuration: 2,
		MaxAudioDuration: 30,
	}, true
}

// SuggestedCapabilities is used only when discovery supplies IDs without metadata.
// A suggestion materializes a preset at catalogue creation; execution never calls it.
func SuggestedCapabilities(id string) []Capability {
	all := []Capability{Story, Image, Video, Speech}
	preset := MigrateLegacyModel(Model{RemoteID: id, Capabilities: all})
	result := []Capability{}
	for _, cap := range all {
		if _, ok := preset.Operations[cap]; ok {
			result = append(result, cap)
		}
	}
	return result
}
