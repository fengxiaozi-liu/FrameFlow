package customjson

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

	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

type Client struct {
	Credentials domain.CredentialReader
	HTTP        *http.Client
}

// render substitutes whole JSON values, preserving numbers, arrays and escaping.
func render(template string, variables map[string]any) (any, error) {
	var body any
	decoder := json.NewDecoder(strings.NewReader(template))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		return nil, err
	}
	var replace func(any) (any, error)
	replace = func(value any) (any, error) {
		switch v := value.(type) {
		case map[string]any:
			for key, item := range v {
				next, err := replace(item)
				if err != nil {
					return nil, err
				}
				v[key] = next
			}
		case []any:
			for i, item := range v {
				next, err := replace(item)
				if err != nil {
					return nil, err
				}
				v[i] = next
			}
		case string:
			if strings.HasPrefix(v, "{{") && strings.HasSuffix(v, "}}") {
				name := v[2 : len(v)-2]
				result, ok := variables[name]
				if !ok {
					return nil, fmt.Errorf("缺少模板变量：%s", name)
				}
				return result, nil
			}
		}
		return value, nil
	}
	return replace(body)
}

func extract(value any, pointer string) (string, error) {
	if !domain.ValidJSONPointer(pointer) {
		return "", errors.New("结果字段路径无效")
	}
	for _, token := range strings.Split(pointer[1:], "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		switch node := value.(type) {
		case map[string]any:
			value = node[token]
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(node) || strconv.Itoa(index) != token {
				return "", fmt.Errorf("结果缺少字段 %s", pointer)
			}
			value = node[index]
		default:
			return "", fmt.Errorf("结果缺少字段 %s", pointer)
		}
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("字段 %s 必须是非空字符串", pointer)
	}
	return text, nil
}

func (c Client) request(ctx context.Context, connection domain.Connection, config domain.CustomInterface, method, path string, body any) (any, error) {
	target, err := connection.RequestURL(path)
	if err != nil {
		return nil, err
	}
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, payload)
	if err != nil {
		return nil, errors.New("无法创建自定义接口请求")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if config.AuthMode != "none" {
		if c.Credentials == nil {
			return nil, errors.New("缺少连接密钥")
		}
		key, err := c.Credentials.Get(ctx, "connection:"+connection.ID)
		if err != nil || key == "" {
			return nil, errors.New("无法读取连接密钥")
		}
		if config.AuthMode == "bearer" {
			req.Header.Set("Authorization", "Bearer "+key)
		} else {
			req.Header.Set(config.AuthHeader, config.AuthPrefix+key)
		}
	}
	httpClient := http.Client{Timeout: 5 * time.Minute}
	if c.HTTP != nil {
		httpClient = *c.HTTP
	}
	// A redirect must not forward credentials to a host outside the connection.
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("自定义接口请求失败，请检查连接地址和网络；未自动重试")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("自定义接口返回 HTTP %d；未自动重试", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil {
		return nil, errors.New("读取接口响应失败")
	}
	if len(raw) > 4<<20 {
		return nil, errors.New("接口响应超过 4 MB")
	}
	var result any
	if json.Unmarshal(raw, &result) != nil {
		return nil, errors.New("接口响应不是有效 JSON")
	}
	return result, nil
}

func mediaURL(value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return "", errors.New("媒体结果必须是 HTTP(S) URL")
	}
	return value, nil
}
func validImage(value string) bool {
	if strings.HasPrefix(value, "data:image/") {
		return true
	}
	_, err := mediaURL(value)
	return err == nil
}
