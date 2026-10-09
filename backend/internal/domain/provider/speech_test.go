package provider

import "testing"

func TestInputCapabilitiesFailClosed(t *testing.T) {
	if _, ok := legacyInputPreset("arbitrary-multi-image"); ok {
		t.Fatal("unknown model accepted")
	}
	c, ok := legacyInputPreset("qwen-image-2.0-pro")
	if !ok || c.ReferenceImages != 3 || c.ReferenceVideo {
		t.Fatal(c)
	}
	c, ok = legacyInputPreset("qwen3-tts-flash")
	if !ok || !c.Speech || len(c.Voices) < 2 {
		t.Fatal(c)
	}
}
