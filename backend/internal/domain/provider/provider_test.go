package provider

import "testing"

func TestConfigLifecycle(t *testing.T) {
	c, e := New("x", "X", Video)
	if e != nil {
		t.Fatal(e)
	}
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
