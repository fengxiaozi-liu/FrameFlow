package provider

import (
	"context"
	"fmt"

	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider/bailian"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider/custom"
)

// DiscoveryRegistry selects the vendor directory only. Model generation retains
// its independent per-operation protocol bindings.
type DiscoveryRegistry struct {
	adapters map[string]domain.Discovery
}

func NewDiscovery() *DiscoveryRegistry {
	return &DiscoveryRegistry{adapters: map[string]domain.Discovery{
		"bailian": bailian.Client{},
		"custom":  custom.DiscoveryClient{},
	}}
}

func (r *DiscoveryRegistry) Discover(ctx context.Context, connection domain.Connection, key string) ([]domain.DiscoveredModel, error) {
	adapter, ok := r.adapters[connection.Vendor]
	if !ok {
		return nil, fmt.Errorf("厂商 %q 尚未接入模型同步，请使用手动添加模型", connection.Vendor)
	}
	return adapter.Discover(ctx, connection, key)
}
