package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// DiscoveryProfile is the bounded, connection-scoped contract used only by
// custom vendors to enumerate remote models. It describes data, never code.
type DiscoveryProfile struct {
	Method              string              `json:"method"`
	Path                string              `json:"path"`
	Headers             map[string]string   `json:"headers,omitempty"`
	RequestJSON         string              `json:"request_json,omitempty"`
	ResponseJSONExample string              `json:"response_json_example,omitempty"`
	ModelListPointer    string              `json:"model_list_pointer"`
	ModelIDPointer      string              `json:"model_id_pointer"`
	ModelNamePointer    string              `json:"model_name_pointer,omitempty"`
	Pagination          DiscoveryPagination `json:"pagination,omitempty"`
	Revision            int64               `json:"revision,omitempty"`
	Schema              *JSONShape          `json:"schema,omitempty"`
}

type DiscoveryPagination struct {
	Mode              string `json:"mode,omitempty"` // none, page, cursor
	PageParam         string `json:"page_param,omitempty"`
	PageSizeParam     string `json:"page_size_param,omitempty"`
	PageSize          int    `json:"page_size,omitempty"`
	CursorParam       string `json:"cursor_param,omitempty"`
	CursorPointer     string `json:"cursor_pointer,omitempty"`
	HasMorePointer    string `json:"has_more_pointer,omitempty"`
	NextCursorPointer string `json:"next_cursor_pointer,omitempty"`
}

const (
	DiscoveryNone   = "none"
	DiscoveryPage   = "page"
	DiscoveryCursor = "cursor"
)

var discoveryHeaderName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,63}$`)

// DefaultCustomDiscoveryProfile preserves the common OpenAI-compatible
// directory while making the path an explicit connection setting in the UI.
func DefaultCustomDiscoveryProfile() DiscoveryProfile {
	return DiscoveryProfile{
		Method:           "GET",
		Path:             "/v1/models",
		ModelListPointer: "/data",
		ModelIDPointer:   "/id",
		Pagination:       DiscoveryPagination{Mode: DiscoveryNone},
	}
}

func (p DiscoveryProfile) Validate() error {
	method := strings.ToUpper(strings.TrimSpace(p.Method))
	if method != "GET" && method != "POST" {
		return errors.New("模型同步接口只支持 GET 或 POST")
	}
	if !ValidCustomPath(p.Path) || strings.ContainsAny(p.Path, "{}") {
		return errors.New("模型同步路径必须是站内相对路径，不能包含域名、查询参数或占位符")
	}
	if !ValidJSONPointer(p.ModelListPointer) || !ValidJSONPointer(p.ModelIDPointer) {
		return errors.New("模型列表和模型 ID 字段必须是有效 JSON Pointer")
	}
	if p.ModelNamePointer != "" && !ValidJSONPointer(p.ModelNamePointer) {
		return errors.New("模型名称字段必须是有效 JSON Pointer")
	}
	if p.RequestJSON != "" {
		if len(p.RequestJSON) > 64<<10 {
			return errors.New("模型同步请求 JSON 不能超过 64 KB")
		}
		var value any
		if err := json.Unmarshal([]byte(p.RequestJSON), &value); err != nil {
			return fmt.Errorf("模型同步请求 JSON 无效：%w", err)
		}
		if method != "POST" {
			return errors.New("GET 模型同步不能配置请求 JSON")
		}
	}
	if len(p.ResponseJSONExample) > 128<<10 {
		return errors.New("模型同步响应示例不能超过 128 KB")
	}
	if p.ResponseJSONExample != "" {
		var value any
		if err := json.Unmarshal([]byte(p.ResponseJSONExample), &value); err != nil {
			return fmt.Errorf("模型同步响应示例无效：%w", err)
		}
	}
	for name, value := range p.Headers {
		if !discoveryHeaderName.MatchString(name) || strings.ContainsAny(value, "\r\n") || strings.EqualFold(name, "host") || strings.EqualFold(name, "content-length") || strings.EqualFold(name, "authorization") || strings.EqualFold(name, "cookie") || strings.EqualFold(name, "connection") || strings.EqualFold(name, "transfer-encoding") {
			return fmt.Errorf("模型同步请求头 %q 无效或不允许覆盖认证头", name)
		}
	}
	return p.Pagination.Validate()
}

func (p DiscoveryPagination) Validate() error {
	mode := p.Mode
	if mode == "" {
		mode = DiscoveryNone
	}
	switch mode {
	case DiscoveryNone:
		return nil
	case DiscoveryPage:
		if !discoveryHeaderName.MatchString(p.PageParam) || p.PageSize <= 0 || p.PageSize > 2000 {
			return errors.New("分页同步需要有效的页码参数和 1 至 2000 的页面大小")
		}
		if p.PageSizeParam != "" && !discoveryHeaderName.MatchString(p.PageSizeParam) {
			return errors.New("分页大小参数名无效")
		}
	case DiscoveryCursor:
		if !discoveryHeaderName.MatchString(p.CursorParam) || p.HasMorePointer == "" || !ValidJSONPointer(p.HasMorePointer) || p.NextCursorPointer == "" || !ValidJSONPointer(p.NextCursorPointer) {
			return errors.New("游标同步需要游标参数、是否有下一页字段和下一游标字段")
		}
	default:
		return errors.New("不支持的模型同步分页方式")
	}
	return nil
}

func (p DiscoveryProfile) Compile() (DiscoveryProfile, error) {
	p.Method = strings.ToUpper(strings.TrimSpace(p.Method))
	if err := p.Validate(); err != nil {
		return p, err
	}
	if p.ResponseJSONExample != "" {
		root, schema, err := ParseJSONExample(p.ResponseJSONExample)
		if err != nil {
			return p, err
		}
		list, err := JSONValue(root, p.ModelListPointer)
		if err != nil {
			return p, err
		}
		items, ok := list.([]any)
		if !ok {
			return p, errors.New("模型列表字段必须指向数组")
		}
		for _, item := range items {
			id, err := JSONValue(item, p.ModelIDPointer)
			if err != nil {
				return p, err
			}
			if text, ok := id.(string); !ok || strings.TrimSpace(text) == "" {
				return p, errors.New("模型 ID 示例必须是非空字符串")
			}
		}
		p.Schema = &schema
	}
	p.Revision = 1
	return p, nil
}
