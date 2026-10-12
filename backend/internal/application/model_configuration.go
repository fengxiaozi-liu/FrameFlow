package application

import (
	"context"
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

// ModelConfiguration coordinates one aggregate update; HTTP/SQL stay outside.
type ModelConfiguration struct{ Repository provider.OperationRepository }

func (s ModelConfiguration) Configure(ctx context.Context, id string, expected int64, bindings map[provider.Capability]provider.OperationBinding, capabilities []provider.Capability) (provider.Model, error) {
	for op, b := range bindings {
		compiled, err := b.Compile(op)
		if err != nil {
			return provider.Model{}, fmt.Errorf("%s: %w", op, err)
		}
		bindings[op] = compiled
	}
	return s.Repository.ConfigureOperations(ctx, id, expected, bindings, capabilities)
}
