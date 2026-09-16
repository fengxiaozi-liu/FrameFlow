package provider

import (
	"context"
	"testing"
)

type testAdapter struct{}

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

func TestRegistryCreatesProviderLazilyAndCachesByCode(t *testing.T) {
	registry := NewRegistry()
	created := 0
	registry.Register("test", func(context.Context, Config) (Adapter, error) {
		created++
		return &testAdapter{}, nil
	})
	config := Config{Code: "test-1", Vendor: "test", Capability: Video}
	if created != 0 {
		t.Fatal("provider factory should not run during registration")
	}
	first, err := registry.Resolve(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.Resolve(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 || first != second {
		t.Fatalf("expected one cached instance, created=%d", created)
	}
}
