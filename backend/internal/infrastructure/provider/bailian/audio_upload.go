package bailian

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

// UploadAudio returns a short-lived signed HTTPS URL for the submitted object.
func (c Client) UploadAudio(ctx context.Context, _ provider.Connection, _ provider.Model, localPath string) (string, error) {
	return c.uploadConfiguredAudio(ctx, localPath)
}
