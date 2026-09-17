package main

import (
	"context"
	"errors"
	"strings"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/security"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
)

// Keep the legacy rows intact so old queued tasks and API clients remain valid.
func migrateProviders(ctx context.Context, store *sqlite.TaskRepository, vault *security.Vault) error {
	for _, capability := range []provider.Capability{provider.Story, provider.Image, provider.Video} {
		items, err := store.ListProviders(ctx, capability)
		if err != nil {
			return err
		}
		for _, old := range items {
			vendor := old.Vendor
			if vendor != "bailian" {
				continue
			}
			connectionID := "legacy-" + old.Code
			_, err := store.GetConnection(ctx, connectionID)
			if errors.Is(err, fault.ErrNotFound) {
				region := "cn-beijing"
				if strings.Contains(old.BaseURL, "dashscope-intl") {
					region = "ap-southeast-1"
				}
				connection := provider.Connection{ID: connectionID, Name: old.Name, Vendor: vendor, Region: region}
				connection.BaseURL = old.BaseURL
				if old.CredentialSet {
					if secret, getErr := vault.Get(ctx, old.Code); getErr == nil {
						if err := vault.Put(ctx, "connection:"+connectionID, secret); err != nil {
							return err
						}
						connection.CredentialSet = true
					}
				}
				if err := store.SaveConnection(ctx, connection); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			if old.Model == "" {
				continue
			}
			id := provider.ModelID(connectionID, old.Model)
			if _, err := store.GetModel(ctx, id); errors.Is(err, fault.ErrNotFound) {
				model := provider.Model{ID: id, ConnectionID: connectionID, RemoteID: old.Model, Name: old.Name, Source: "manual", Capabilities: []provider.Capability{capability}}
				if err := store.SaveModel(ctx, model); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}
	}
	return nil
}
