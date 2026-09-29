package provider

import "testing"

func TestInputCapabilitiesFailClosed(t *testing.T) {
	if _, ok := InputCapabilityFor("arbitrary-multi-image"); ok {
		t.Fatal("unknown model accepted")
	}
	c, ok := InputCapabilityFor("qwen-image-2.0-pro")
	if !ok || c.ReferenceImages != 3 || c.ReferenceVideo {
		t.Fatal(c)
	}
	c, ok = InputCapabilityFor("qwen3-tts-flash")
	if !ok || !c.Speech || len(c.Voices) < 2 {
		t.Fatal(c)
	}
}
