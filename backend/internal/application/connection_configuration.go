package application

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

type ConnectionConfiguration struct {
	Repository provider.ConnectionRepository
	Vault      provider.CredentialVault
	mu         sync.Mutex
}

func (s *ConnectionConfiguration) Save(ctx context.Context, c provider.Connection, key string, create bool) (provider.Connection, error) {
	// Serialize configuration/credential coordination within this command service.
	// Network discovery is a separate use case and does not hold this lock.
	s.mu.Lock()
	defer s.mu.Unlock()
	if create {
		_, err := s.Repository.GetConnection(ctx, c.ID)
		if err == nil {
			return c, provider.ErrRevisionConflict
		}
		if !errors.Is(err, fault.ErrNotFound) {
			return c, err
		}
	}
	if !create {
		current, err := s.Repository.GetConnection(ctx, c.ID)
		if err != nil {
			return c, err
		}
		if current.Vendor != c.Vendor {
			return c, errors.New("vendor cannot change; create a new connection")
		}
		if current.Revision != c.Revision {
			return c, provider.ErrRevisionConflict
		}
	}
	if c.Vendor != "custom" && c.Discovery != nil {
		return c, errors.New("内置厂商使用固定模型目录，不需要自定义同步配置")
	}
	if c.Vendor == "custom" {
		profile := provider.DefaultCustomDiscoveryProfile()
		if c.Discovery != nil {
			profile = *c.Discovery
		}
		compiled, err := profile.Compile()
		if err != nil {
			return c, err
		}
		c.Discovery = &compiled
	}
	if err := c.Validate(); err != nil {
		return c, err
	}
	c.BaseURL = strings.TrimSuffix(strings.TrimSpace(c.BaseURL), "/")
	c.SchemaVersion = 1
	credentialID := "connection:" + c.ID
	var previousKey string
	var previousSet bool
	if s.Vault != nil {
		var err error
		previousSet, err = s.Vault.Has(ctx, credentialID)
		if err != nil {
			return c, err
		}
		if key != "" {
			if previousSet {
				reader, ok := s.Vault.(provider.CredentialReader)
				if !ok {
					return c, errors.New("凭据设施不支持安全更新")
				}
				previousKey, err = reader.Get(ctx, credentialID)
				if err != nil {
					return c, err
				}
			}
			if err := s.Vault.Put(ctx, credentialID, key); err != nil {
				return c, err
			}
		}
		c.CredentialSet = previousSet || key != ""
	}
	if err := s.Repository.SaveConnection(ctx, c); err != nil {
		if key != "" && s.Vault != nil {
			recoveryContext := context.WithoutCancel(ctx)
			var recoveryErr error
			if previousSet {
				recoveryErr = s.Vault.Put(recoveryContext, credentialID, previousKey)
			} else {
				recoveryErr = s.Vault.Delete(recoveryContext, credentialID)
			}
			if recoveryErr != nil {
				return c, errors.New("连接保存失败且凭据恢复失败，请检查连接密钥")
			}
		}
		return c, err
	}
	return s.Repository.GetConnection(ctx, c.ID)
}
