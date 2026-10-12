package custom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

const (
	maxResponseBytes = 4 << 20
	maxPages         = 100
)

// DiscoveryClient discovers model identity from a user-configured, bounded JSON
// directory contract. It never infers generation capabilities.
type DiscoveryClient struct {
	HTTP *http.Client
}

func (c DiscoveryClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c DiscoveryClient) Discover(ctx context.Context, connection provider.Connection, key string) ([]provider.DiscoveredModel, error) {
	if connection.Discovery == nil {
		return c.discoverCompatible(ctx, connection, key)
	}
	profile := *connection.Discovery
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	items := make([]provider.DiscoveredModel, 0)
	seen := map[string]bool{}
	seenCursors := map[string]bool{}
	cursor := ""
	for page := 1; page <= maxPages; page++ {
		path, err := discoveryPath(profile, page, cursor)
		if err != nil {
			return nil, err
		}
		value, err := c.request(ctx, connection, key, profile, path)
		if err != nil {
			return nil, err
		}
		list, err := pointer(value, profile.ModelListPointer)
		if err != nil {
			return nil, fmt.Errorf("模型同步列表字段解析失败：%w", err)
		}
		entries, ok := list.([]any)
		if !ok {
			return nil, errors.New("模型同步列表字段必须是数组")
		}
		before := len(items)
		for _, raw := range entries {
			idValue, err := pointer(raw, profile.ModelIDPointer)
			if err != nil {
				return nil, fmt.Errorf("模型同步条目缺少模型 ID：%w", err)
			}
			id, ok := idValue.(string)
			id = strings.TrimSpace(id)
			if !ok || id == "" || strings.ContainsAny(id, "\r\n\x00") {
				return nil, errors.New("模型同步条目的模型 ID 必须是非空字符串")
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			name := id
			if profile.ModelNamePointer != "" {
				if value, lookupErr := pointer(raw, profile.ModelNamePointer); lookupErr == nil {
					if candidate, ok := value.(string); ok && strings.TrimSpace(candidate) != "" {
						name = strings.TrimSpace(candidate)
					}
				}
			}
			items = append(items, provider.DiscoveredModel{ID: id, Name: name})
		}
		more, next, done, err := nextPage(value, profile.Pagination, len(entries))
		if err != nil {
			return nil, err
		}
		if done || len(entries) == 0 {
			return items, nil
		}
		if more {
			if next == "" || (profile.Pagination.Mode == provider.DiscoveryCursor && seenCursors[next]) || len(items) == before {
				return nil, errors.New("模型同步分页没有推进游标")
			}
			seenCursors[next] = true
			cursor = next
			continue
		}
		return items, nil
	}
	return nil, errors.New("模型同步分页超过安全上限")
}

func discoveryPath(profile provider.DiscoveryProfile, page int, cursor string) (string, error) {
	path := profile.Path
	mode := profile.Pagination.Mode
	if mode == "" {
		mode = provider.DiscoveryNone
	}
	query := url.Values{}
	switch mode {
	case provider.DiscoveryPage:
		query.Set(profile.Pagination.PageParam, strconv.Itoa(page))
		if profile.Pagination.PageSizeParam != "" {
			query.Set(profile.Pagination.PageSizeParam, strconv.Itoa(profile.Pagination.PageSize))
		}
	case provider.DiscoveryCursor:
		if cursor != "" {
			query.Set(profile.Pagination.CursorParam, cursor)
		}
	case provider.DiscoveryNone:
	default:
		return "", errors.New("不支持的模型同步分页方式")
	}
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path, nil
}

func nextPage(root any, pagination provider.DiscoveryPagination, itemCount int) (more bool, next string, done bool, err error) {
	mode := pagination.Mode
	if mode == "" || mode == provider.DiscoveryNone {
		return false, "", true, nil
	}
	if mode == provider.DiscoveryPage {
		// Page mode stops when a page is short; an exact full page is followed by
		// one bounded request so a service without total metadata still works.
		if itemCount < pagination.PageSize {
			return false, "", true, nil
		}
		return true, "page", false, nil
	}
	hasMore, lookupErr := pointer(root, pagination.HasMorePointer)
	if lookupErr != nil {
		return false, "", false, fmt.Errorf("分页是否有更多字段解析失败：%w", lookupErr)
	}
	flag, ok := hasMore.(bool)
	if !ok {
		return false, "", false, errors.New("分页是否有更多字段必须是布尔值")
	}
	if !flag {
		return false, "", true, nil
	}
	nextValue, lookupErr := pointer(root, pagination.NextCursorPointer)
	if lookupErr != nil {
		return false, "", false, fmt.Errorf("下一游标字段解析失败：%w", lookupErr)
	}
	next, ok = nextValue.(string)
	if !ok || strings.TrimSpace(next) == "" {
		return false, "", false, errors.New("下一游标必须是非空字符串")
	}
	return true, next, false, nil
}

func (c DiscoveryClient) request(ctx context.Context, connection provider.Connection, key string, profile provider.DiscoveryProfile, path string) (any, error) {
	raw, err := c.requestJSON(ctx, connection, key, profile, path)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, errors.New("模型同步响应不是有效 JSON")
	}
	return value, nil
}

func (c DiscoveryClient) requestJSON(ctx context.Context, connection provider.Connection, key string, profile provider.DiscoveryProfile, path string) ([]byte, error) {
	target, err := connection.RequestURL(path)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if profile.Method == http.MethodPost && profile.RequestJSON != "" {
		body = bytes.NewReader([]byte(profile.RequestJSON))
	}
	req, err := http.NewRequestWithContext(ctx, profile.Method, target, body)
	if err != nil {
		return nil, err
	}
	auth := connection.Authentication()
	if auth.Mode == "bearer" {
		req.Header.Set("Authorization", "Bearer "+key)
	} else if auth.Mode == "header" {
		req.Header.Set(auth.Header, auth.Prefix+key)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, value := range profile.Headers {
		if auth.Mode == "header" && strings.EqualFold(name, auth.Header) {
			return nil, errors.New("模型同步请求头不能覆盖连接认证头")
		}
		req.Header.Set(name, value)
	}
	client := *c.httpClient()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("模型同步请求失败，请检查连接地址和网络")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("模型同步接口返回 HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, errors.New("读取模型同步响应失败")
	}
	if len(raw) > maxResponseBytes {
		return nil, errors.New("模型同步响应超过 4 MB")
	}
	return raw, nil
}

func pointer(root any, path string) (any, error) {
	if !provider.ValidJSONPointer(path) {
		return nil, errors.New("JSON Pointer 无效")
	}
	current := root
	for _, token := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		switch value := current.(type) {
		case map[string]any:
			next, ok := value[token]
			if !ok {
				return nil, fmt.Errorf("字段 %q 不存在", token)
			}
			current = next
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(value) {
				return nil, fmt.Errorf("数组索引 %q 无效", token)
			}
			current = value[index]
		default:
			return nil, fmt.Errorf("无法从 %T 继续读取字段 %q", current, token)
		}
	}
	return current, nil
}
