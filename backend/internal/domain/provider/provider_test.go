package provider

import (
	"testing"
)

func TestConfigLifecycle(t *testing.T) {
	c := Config{Code: "x", Name: "X", Capability: Video, Status: Disabled}
	c.Enable()
	c.MarkHealthy()
	if c.Status != Healthy || !c.Enabled {
		t.Fatal(c)
	}
	c.Disable()
	if c.Enabled {
		t.Fatal(c)
	}
}
