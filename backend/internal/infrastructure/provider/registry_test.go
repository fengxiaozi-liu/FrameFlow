package provider

import (
	"context"
	"strings"
	"testing"

	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

func TestRegistryRegistersBailianAndRejectsUnknownVendor(t *testing.T) {
	r := NewRegistry(nil)
	if _, err := r.client(domain.Connection{Vendor: "bailian"}); err != nil {
		t.Fatal(err)
	}
	_, err := r.GenerateImage(context.Background(), domain.Connection{Vendor: "unknown"}, domain.Model{}, domain.ImageRequest{}, domain.RequestOptions{})
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatal(err)
	}
}
