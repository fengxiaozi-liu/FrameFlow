package provider

import "testing"

func TestWan27VideoCapability(t *testing.T) {
	c, ok := VideoCapabilityFor("wan2.7-i2v-2026-04-25")
	if !ok || c.MinDuration != 2 || c.MaxDuration != 15 || c.MaxImageBytes != 20<<20 || c.MaxAudioBytes != 15<<20 {
		t.Fatalf("unexpected capability: %+v, %v", c, ok)
	}
	valid := []string{"first_frame", "last_frame", "driving_audio"}
	if err := c.ValidateInput(10, "720P", valid); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		duration   int
		resolution string
		usages     []string
	}{
		{"no first frame", 5, "720P", []string{"last_frame"}},
		{"duplicate first frame", 5, "720P", []string{"first_frame", "first_frame"}},
		{"reference image", 5, "720P", []string{"first_frame", "character_reference"}},
		{"short duration", 1, "720P", []string{"first_frame"}},
		{"long duration", 16, "720P", []string{"first_frame"}},
		{"bad resolution", 5, "480P", []string{"first_frame"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := c.ValidateInput(tc.duration, tc.resolution, tc.usages); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestUnknownVideoModelIsUnsupported(t *testing.T) {
	if _, ok := VideoCapabilityFor("other-video-model"); ok {
		t.Fatal("unknown model must not acquire an implicit capability")
	}
}
