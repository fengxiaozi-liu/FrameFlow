package bailian

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

// Discover reads the native directory. Generation endpoints never select a directory.
func (c Client) Discover(ctx context.Context, connection provider.Connection, key string) ([]provider.DiscoveredModel, error) {
	items := make([]provider.DiscoveredModel, 0)
	seen := map[string]bool{}
	for page := 1; page <= 100; page++ {
		var response struct {
			Success *bool `json:"success"`
			Output  *struct {
				Total  *int `json:"total"`
				Models []struct {
					Model        string          `json:"model"`
					Name         string          `json:"name"`
					Capabilities json.RawMessage `json:"capabilities"`
				} `json:"models"`
			} `json:"output"`
		}
		path := fmt.Sprintf("/api/v1/models?page_no=%d&page_size=100", page)
		if err := c.request(ctx, connection, key, http.MethodGet, path, nil, &response, false); err != nil {
			return nil, err
		}
		if response.Success != nil && !*response.Success {
			return nil, errors.New("模型目录查询失败，请检查所选厂商和连接权限")
		}
		if response.Output == nil || response.Output.Total == nil || *response.Output.Total < 0 || response.Output.Models == nil {
			return nil, errors.New("模型目录响应格式不符，预期百炼原生 output.models 和 total")
		}
		before := len(items)
		for _, model := range response.Output.Models {
			if model.Model == "" {
				return nil, errors.New("模型目录条目缺少模型 ID")
			}
			if seen[model.Model] {
				continue
			}
			seen[model.Model] = true
			name := model.Name
			if name == "" {
				name = model.Model
			}
			entry := provider.DiscoveredModel{ID: model.Model, Name: name}
			var capabilities []string
			if json.Unmarshal(model.Capabilities, &capabilities) == nil {
				for _, code := range capabilities {
					var capability provider.Capability
					switch code {
					case "TG", string(provider.Story):
						capability = provider.Story
					case "IG", string(provider.Image):
						capability = provider.Image
					case "VG", string(provider.Video):
						capability = provider.Video
					case "TTS", string(provider.Speech):
						capability = provider.Speech
					}
					if capability != "" && !slices.Contains(entry.Capabilities, capability) {
						entry.Capabilities = append(entry.Capabilities, capability)
					}
				}
			}
			items = append(items, entry)
		}
		if len(items) >= *response.Output.Total {
			return items, nil
		}
		if len(items) == before {
			return nil, errors.New("模型目录分页未返回新条目，同步未完成，请重试")
		}
	}
	return nil, errors.New("model list exceeds pagination limit")
}

// CompatibleDiscovery uses an explicitly selected OpenAI-compatible directory.
// An ID-only response does not declare any generation capabilities.
type CompatibleDiscovery struct{ HTTP *http.Client }

func (d CompatibleDiscovery) Discover(ctx context.Context, connection provider.Connection, key string) ([]provider.DiscoveredModel, error) {
	c := Client{HTTP: d.HTTP}
	items := make([]provider.DiscoveredModel, 0)
	seen := map[string]bool{}
	cursors := map[string]bool{}
	path := "/v1/models"
	for page := 0; page < 100; page++ {
		var response struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		if err := c.request(ctx, connection, key, http.MethodGet, path, nil, &response, false); err != nil {
			return nil, err
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
