package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const CustomProtocol = "custom.json.v1"

// CustomInterface describes a bounded JSON wire contract, never executable code.
type CustomInterface struct {
	Path          string         `json:"path"`
	BodyTemplate  string         `json:"body_template"`
	AuthMode      string         `json:"auth_mode"`
	AuthHeader    string         `json:"auth_header,omitempty"`
	AuthPrefix    string         `json:"auth_prefix,omitempty"`
	ResultPointer string         `json:"result_pointer"`
	Mode          string         `json:"mode"`
	Polling       *CustomPolling `json:"polling,omitempty"`
}
type CustomPolling struct {
	Path          string   `json:"path"`
	TaskIDPointer string   `json:"task_id_pointer"`
	StatusPointer string   `json:"status_pointer"`
	Pending       []string `json:"pending"`
	Succeeded     []string `json:"succeeded"`
	Failed        []string `json:"failed"`
}

func CustomVariables(op Capability) []string {
	variables := []string{"model"}
	switch op {
	case Story:
		return append(variables, "prompt", "system_prompt", "messages")
	case Image:
		return append(variables, "prompt", "images", "ratio", "size")
	case Video:
		return append(variables, "prompt", "images", "first_frame", "last_frame", "duration", "resolution", "ratio")
	case Speech:
		return append(variables, "text", "voice", "language", "instructions")
	}
	return variables
}

// ValidCustomPath keeps requests on the configured connection host.
func ValidCustomPath(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") && !strings.ContainsAny(path, "?#\\%\r\n\t ") && !strings.Contains(path, "..") && len(path) <= 2048
}
func ValidJSONPointer(pointer string) bool {
	if !strings.HasPrefix(pointer, "/") || len(pointer) > 1024 {
		return false
	}
	for i := 0; i < len(pointer); i++ {
		if pointer[i] == '~' {
			if i+1 >= len(pointer) || (pointer[i+1] != '0' && pointer[i+1] != '1') {
				return false
			}
			i++
		}
	}
	return true
}

var headerName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,63}$`)

func (c CustomInterface) Variables() (map[string]bool, error) {
	if len(c.BodyTemplate) > 65536 {
		return nil, errors.New("请求模板不能超过 64 KB")
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(c.BodyTemplate), &body); err != nil || body == nil {
		return nil, errors.New("请求模板必须是 JSON 对象")
	}
	used := map[string]bool{}
	var walk func(any) error
	walk = func(value any) error {
		switch v := value.(type) {
		case map[string]any:
			for key, item := range v {
				if strings.Contains(key, "{{") {
					return errors.New("变量只能放在 JSON 值中")
				}
				if err := walk(item); err != nil {
					return err
				}
			}
		case []any:
			for _, item := range v {
				if err := walk(item); err != nil {
					return err
				}
			}
		case string:
			if strings.Contains(v, "{{") || strings.Contains(v, "}}") {
				if !strings.HasPrefix(v, "{{") || !strings.HasSuffix(v, "}}") {
					return errors.New("模板变量必须占据整个 JSON 字符串值")
				}
				name := strings.TrimSuffix(strings.TrimPrefix(v, "{{"), "}}")
				used[name] = true
			}
		}
		return nil
	}
	return used, walk(body)
}
func (c CustomInterface) Validate(op Capability) error {
	if !ValidCustomPath(c.Path) || strings.ContainsAny(c.Path, "{}") {
		return errors.New("自定义请求路径必须以 / 开头，不含域名、查询参数或占位符")
	}
	switch c.AuthMode {
	case "none", "bearer":
	case "header":
		if !headerName.MatchString(c.AuthHeader) || slices.Contains([]string{"host", "content-length", "content-type", "connection", "cookie", "transfer-encoding"}, strings.ToLower(c.AuthHeader)) || strings.ContainsAny(c.AuthPrefix, "\r\n") || len(c.AuthPrefix) > 64 {
			return errors.New("认证请求头或前缀无效")
		}
	default:
		return errors.New("请选择自定义接口的认证方式")
	}
	if !ValidJSONPointer(c.ResultPointer) {
		return errors.New("请填写以 / 开头的结果字段路径，例如 /data/0/url")
	}
	used, err := c.Variables()
	if err != nil {
		return err
	}
	for name := range used {
		if !slices.Contains(CustomVariables(op), name) {
			return fmt.Errorf("此能力不支持模板变量：%s", name)
		}
	}
	if !used["model"] {
		return errors.New("请求模板需要包含 {{model}}")
	}
	if op == Speech {
		if !used["text"] || !used["voice"] {
			return errors.New("语音模板需要包含 {{text}} 和 {{voice}}")
		}
	} else if !used["prompt"] && !(op == Story && used["messages"]) {
		return errors.New("请求模板需要包含 {{prompt}} 或文本能力的 {{messages}}")
	}
	if c.Mode == "sync" {
		return nil
	}
	if c.Mode != "async" || op != Video {
		return errors.New("当前仅视频支持异步查询，其他能力请选择同步返回")
	}
	p := c.Polling
	if p == nil || !ValidCustomPath(p.Path) || strings.Count(p.Path, "{task_id}") != 1 || strings.ContainsAny(strings.ReplaceAll(p.Path, "{task_id}", ""), "{}") || !ValidJSONPointer(p.TaskIDPointer) || !ValidJSONPointer(p.StatusPointer) {
		return errors.New("请配置任务 ID、状态字段与包含 {task_id} 的查询路径")
	}
	seen := map[string]bool{}
	for _, states := range [][]string{p.Pending, p.Succeeded, p.Failed} {
		if len(states) == 0 {
			return errors.New("等待、成功与失败状态均须配置")
		}
		for _, state := range states {
			if strings.TrimSpace(state) == "" || seen[state] {
				return errors.New("状态值不能为空或在多个分组重复")
			}
			seen[state] = true
		}
	}
	return nil
}
