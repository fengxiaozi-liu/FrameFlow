package provider

import (
	"fmt"
	"slices"
)

// VideoCapability describes inputs verified for one provider protocol. A model
// without an explicit entry is not executable for scene video generation.
type VideoCapability struct {
	InputMode        string         `json:"input_mode"`
	Durations        []int          `json:"durations,omitempty"`
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

// ValidateInput rejects unknown and unsupported media instead of dropping them.
func (c VideoCapability) ValidateInput(duration int, resolution string, usages []string) error {
	if c.ModelPrefix == "" {
		return fmt.Errorf("video model capability is unknown")
	}
	if duration < c.MinDuration || duration > c.MaxDuration || len(c.Durations) > 0 && !slices.Contains(c.Durations, duration) {
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
	if c.InputMode == "reference_images" {
		if counts["reference_image"] == 0 {
			return fmt.Errorf("at least one reference_image is required")
		}
	} else if counts["first_frame"] != 1 {
		return fmt.Errorf("exactly one first_frame is required")
	}
	return nil
}
