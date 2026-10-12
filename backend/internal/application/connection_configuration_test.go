package application

import (
	"context"
	"errors"
	"testing"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

type configurationRepository struct {
	provider.ConnectionRepository
	current   provider.Connection
	exists    bool
	saveError error
}

func (r *configurationRepository) GetConnection(context.Context, string) (provider.Connection, error) {
	if !r.exists {
		return provider.Connection{}, fault.ErrNotFound
	}
	return r.current, nil
}

func (r *configurationRepository) SaveConnection(_ context.Context, c provider.Connection) error {
	if r.saveError != nil {
		return r.saveError
	}
	c.Revision++
	r.current, r.exists = c, true
	return nil
}

type configurationVault struct{ value string }

func (v *configurationVault) Has(context.Context, string) (bool, error)   { return v.value != "", nil }
func (v *configurationVault) Get(context.Context, string) (string, error) { return v.value, nil }
func (v *configurationVault) Put(_ context.Context, _ string, value string) error {
	v.value = value
	return nil
}
func (v *configurationVault) Delete(context.Context, string) error { v.value = ""; return nil }

func TestConnectionSaveFailureRestoresCredential(t *testing.T) {
	for _, existing := range []bool{false, true} {
		c := provider.Connection{ID: "c", Name: "Custom", Vendor: "custom", BaseURL: "https://example.com", Revision: 1}
		repo := &configurationRepository{current: c, exists: existing, saveError: errors.New("synthetic write failure")}
		vault := &configurationVault{}
		if existing {
			vault.value = "old-local-placeholder"
		}
		previous := vault.value
		service := ConnectionConfiguration{Repository: repo, Vault: vault}
		if _, err := service.Save(context.Background(), c, "new-local-placeholder", !existing); err == nil {
			t.Fatal("save failure ignored")
		}
		if vault.value != previous {
			t.Fatal("credential changed despite failed configuration save")
		}
	}
}

func TestDuplicateConnectionCreateDoesNotRotateCredential(t *testing.T) {
	c := provider.Connection{ID: "c", Name: "Custom", Vendor: "custom", BaseURL: "https://example.com", Revision: 1}
	vault := &configurationVault{value: "old-local-placeholder"}
	service := ConnectionConfiguration{Repository: &configurationRepository{current: c, exists: true}, Vault: vault}
	if _, err := service.Save(context.Background(), c, "new-local-placeholder", true); !errors.Is(err, provider.ErrRevisionConflict) {
		t.Fatal(err)
	}
	if vault.value != "old-local-placeholder" {
		t.Fatal("duplicate create rotated credentials")
	}
}

func TestConnectionSaveCompilesCustomDirectoryAndRejectsBuiltinOverride(t *testing.T) {
	c := provider.Connection{ID: "c", Name: "Custom", Vendor: "custom", BaseURL: "https://example.com"}
	service := ConnectionConfiguration{Repository: &configurationRepository{}}
	saved, err := service.Save(context.Background(), c, "", true)
	if err != nil || saved.Discovery == nil || saved.Discovery.Path != "/v1/models" || saved.Revision != 1 {
		t.Fatal(saved, err)
	}
	c.ID, c.Vendor, c.Discovery = "builtin", "bailian", saved.Discovery
	service.Repository = &configurationRepository{}
	if _, err := service.Save(context.Background(), c, "", true); err == nil {
		t.Fatal("builtin directory override accepted")
	}
}
