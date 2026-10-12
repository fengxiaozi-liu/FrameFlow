package custom

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

// discoverCompatible preserves cursor discovery for connections saved before
// explicit directory profiles. Generation capabilities remain model settings.
func (d DiscoveryClient) discoverCompatible(ctx context.Context, connection provider.Connection, key string) ([]provider.DiscoveredModel, error) {
	items := make([]provider.DiscoveredModel, 0)
	seen := map[string]bool{}
	cursors := map[string]bool{}
	path := "/v1/models"
	for page := 0; page < maxPages; page++ {
		var response struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		raw, err := d.requestJSON(ctx, connection, key, provider.DefaultCustomDiscoveryProfile(), path)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, errors.New("模型同步响应不是有效的兼容目录 JSON")
		}
		if response.Data == nil {
			return nil, errors.New("模型目录响应格式不符，预期 OpenAI 兼容 data 数组")
		}
		for _, model := range response.Data {
			if model.ID == "" {
				return nil, errors.New("模型目录条目缺少模型 ID")
			}
			if !seen[model.ID] {
				seen[model.ID] = true
				items = append(items, provider.DiscoveredModel{ID: model.ID, Name: model.ID})
			}
		}
		if !response.HasMore {
			return items, nil
		}
		if response.LastID == "" || cursors[response.LastID] {
			return nil, errors.New("模型目录分页缺少有效游标，同步未完成")
		}
		cursors[response.LastID] = true
		path = "/v1/models?after=" + url.QueryEscape(response.LastID)
	}
	return nil, errors.New("model list exceeds pagination limit")
}
