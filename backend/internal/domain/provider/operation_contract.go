package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

type ExecutionMode string

const (
	SyncJSON     ExecutionMode = "sync_json"
	AsyncPolling ExecutionMode = "async_polling"
	SSEStream    ExecutionMode = "sse_stream"
	NDJSONStream ExecutionMode = "ndjson_stream"
)

// OperationContract belongs to exactly one model capability. Discovery cannot
// create this contract; user-confirmed bindings give fields their meaning.
type OperationContract struct {
	Version         int           `json:"version"`
	Request         JSONContract  `json:"request"`
	Response        JSONContract  `json:"response"`
	PollingResponse *JSONContract `json:"polling_response,omitempty"`
	Execution       ExecutionMode `json:"execution"`
	DoneMarker      string        `json:"done_marker,omitempty"`
	ErrorPointer    string        `json:"error_pointer,omitempty"`
}

func (c OperationContract) Compile(op Capability, transport CustomInterface) (OperationContract, CustomInterface, error) {
	if c.Version != 0 && c.Version != 1 {
		return c, transport, errors.New("不支持的 JSON 契约版本")
	}
	var err error
	c.Request, err = c.Request.Compile()
	if err != nil {
		return c, transport, fmt.Errorf("请求示例：%w", err)
	}
	c.Response, err = c.Response.Compile()
	if err != nil {
		return c, transport, fmt.Errorf("响应示例：%w", err)
	}
	root, _, _ := ParseJSONExample(c.Request.Example)
	seen := map[string]bool{}
	for name, pointer := range c.Request.Bindings {
		if !slices.Contains(CustomVariables(op), name) {
			return c, transport, fmt.Errorf("不支持的请求变量 %s", name)
		}
		value, err := JSONValue(root, pointer)
		if err != nil {
			return c, transport, err
		}
		validType := false
		switch name {
		case "messages", "images":
			_, validType = value.([]any)
		case "duration":
			_, validType = value.(float64)
		default:
			_, validType = value.(string)
		}
		if !validType {
			return c, transport, fmt.Errorf("请求字段 %s 的示例类型与业务变量不符", name)
		}
		for other := range seen {
			if pointer == other || strings.HasPrefix(pointer, other+"/") || strings.HasPrefix(other, pointer+"/") {
				return c, transport, errors.New("请求字段不能重复映射或同时映射父节点与子节点")
			}
		}
		seen[pointer] = true
	}
	for name, pointer := range c.Request.Bindings {
		if err := ReplaceJSONValue(root, pointer, "{{"+name+"}}"); err != nil {
			return c, transport, err
		}
	}
	body, err := json.Marshal(root)
	if err != nil {
		return c, transport, err
	}
	transport.BodyTemplate = string(body)
	switch c.Execution {
	case SyncJSON, SSEStream, NDJSONStream:
		if err := c.Response.RequireStringBindings("result"); err != nil {
			return c, transport, err
		}
		transport.Mode = "sync"
		transport.ResultPointer = c.Response.Bindings["result"]
		if transport.ResultPointer == "" {
			return c, transport, errors.New("请映射响应结果字段")
		}
	case AsyncPolling:
		if err := c.Response.RequireStringBindings("task_id"); err != nil {
			return c, transport, err
		}
		if op != Video {
			return c, transport, errors.New("当前异步任务入口只支持视频；其他能力可使用同步或流式响应")
		}
		if c.PollingResponse == nil || transport.Polling == nil {
			return c, transport, errors.New("请配置查询响应示例与查询路径")
		}
		compiled, e := c.PollingResponse.Compile()
		if e != nil {
			return c, transport, e
		}
		c.PollingResponse = &compiled
		if err := compiled.RequireStringBindings("status", "result"); err != nil {
			return c, transport, err
		}
		transport.Mode = "async"
		transport.Polling.TaskIDPointer = c.Response.Bindings["task_id"]
		transport.Polling.StatusPointer = compiled.Bindings["status"]
		transport.ResultPointer = compiled.Bindings["result"]
	default:
		return c, transport, errors.New("不支持的执行方式")
	}
	if c.ErrorPointer != "" && !ValidJSONPointer(c.ErrorPointer) {
		return c, transport, errors.New("错误字段映射无效")
	}
	c.Version = 1
	transport.StreamMode = ""
	if c.Execution == SSEStream || c.Execution == NDJSONStream {
		transport.StreamMode = c.Execution
		transport.StreamDoneMarker = c.DoneMarker
		transport.StreamErrorPointer = c.ErrorPointer
		transport.StreamAppend = op == Story
	}
	if err := transport.Validate(op); err != nil {
		return c, transport, err
	}
	return c, transport, nil
}

// CompileBinding freezes example-derived schemas while preserving the legacy
// transport representation for already-created tasks and built-in protocols.
func (b OperationBinding) Compile(op Capability) (OperationBinding, error) {
	if op == Image {
		b = b.ResolvedImageBinding()
	}
	if b.Contract == nil {
		return b, b.Validate(op)
	}
	if b.Protocol != CustomProtocol || b.Custom == nil {
		return b, errors.New("JSON 示例契约只能用于自定义接口")
	}
	c, transport, err := b.Contract.Compile(op, *b.Custom)
	if err != nil {
		return b, err
	}
	b.Contract = &c
	b.Custom = &transport
	return b, b.Validate(op)
}
