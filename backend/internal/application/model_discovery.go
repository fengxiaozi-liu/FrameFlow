package application

import (
	"context"
	"errors"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

// ModelIdentityWriter atomically merges directory identities against the latest
// stored aggregates, rather than overwriting a concurrent model configuration.
type ModelIdentityWriter interface {
	SyncModelIdentities(context.Context, provider.Connection, []provider.DiscoveredModel, time.Time) (int, error)
}

type ModelDiscovery struct {
	Connections provider.ConnectionRepository
	Writer      ModelIdentityWriter
	Discovery   provider.Discovery
	Credentials provider.CredentialReader
}

func (s ModelDiscovery) Sync(ctx context.Context, id string) (int, error) {
	c, err := s.Connections.GetConnection(ctx, id)
	if err != nil {
		return 0, err
	}
	if s.Writer == nil || s.Discovery == nil || s.Credentials == nil {
		return 0, errors.New("模型同步服务不可用")
	}
	key := ""
	if c.Authentication().Mode != "none" {
		key, err = s.Credentials.Get(ctx, "connection:"+id)
		if err != nil {
			return 0, errors.New("无法读取连接凭据")
		}
	}
	entries, err := s.Discovery.Discover(ctx, c, key)
	if err != nil {
		return 0, err
	}
	return s.Writer.SyncModelIdentities(ctx, c, entries, time.Now().UTC())
}
