package provider

import (
	"fmt"
)

// VideoCapability describes inputs verified for one provider protocol. A model
// without an explicit entry is not executable for scene video generation.
type VideoCapability struct {
	ModelPrefix      string         `json:"model_prefix"`
	MinDuration      int            `json:"min_duration"`
	MaxDuration      int            `json:"max_duration"`
	Resolutions      []string       `json:"resolutions"`
	AllowedMedia     map[string]int `json:"allowed_media"`
	MaxImageBytes    int64          `json:"max_image_bytes"`
	MaxAudioBytes    int64          `json:"max_audio_bytes"`
	MinAudioDuration int            `json:"min_audio_duration"`
	MaxAudioDuration int            `json:"max_audio_duration"`
}

func VideoCapabilityFor(remoteModelID string) (VideoCapability, bool) {
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

// ValidateInput rejects unknown and unsupported media instead of dropping them.
func (c VideoCapability) ValidateInput(duration int, resolution string, usages []string) error {
	if c.ModelPrefix == "" {
		return fmt.Errorf("video model capability is unknown")
	}
	if duration < c.MinDuration || duration > c.MaxDuration {
		return fmt.Errorf("duration must be %d-%d seconds", c.MinDuration, c.MaxDuration)
	}
	validResolution := false
	for _, allowed := range c.Resolutions {
		if allowed == resolution {
			validResolution = true
			break
		}
	}
	if !validResolution {
		return fmt.Errorf("resolution %q is not supported", resolution)
	}
	counts := make(map[string]int)
	for _, usage := range usages {
		limit, supported := c.AllowedMedia[usage]
		if !supported {
			return fmt.Errorf("media usage %q is not supported", usage)
		}
		counts[usage]++
		if counts[usage] > limit {
			return fmt.Errorf("media usage %q exceeds limit %d", usage, limit)
		}
	}
	if counts["first_frame"] != 1 {
		return fmt.Errorf("exactly one first_frame is required")
	}
	return nil
}
