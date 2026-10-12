package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// JSONShape is a bounded schema inferred from examples. Example values never
// determine capabilities or semantic field meanings; mappings do that.
type JSONShape struct {
	Type       string               `json:"type"`
	Properties map[string]JSONShape `json:"properties,omitempty"`
	Items      *JSONShape           `json:"items,omitempty"`
}

type JSONContract struct {
	Example  string            `json:"example"`
	Schema   *JSONShape        `json:"schema,omitempty"`
	Bindings map[string]string `json:"bindings"`
}

func ParseJSONExample(example string) (any, JSONShape, error) {
	if len(example) == 0 || len(example) > 128<<10 {
		return nil, JSONShape{}, errors.New("JSON 示例须为 1 至 128 KB")
	}
	var value any
	if err := json.Unmarshal([]byte(example), &value); err != nil {
		return nil, JSONShape{}, errors.New("请输入有效 JSON 示例")
	}
	nodes := 0
	var infer func(any, int) (JSONShape, error)
	infer = func(value any, depth int) (JSONShape, error) {
		nodes++
		if depth > 32 || nodes > 4096 {
			return JSONShape{}, errors.New("JSON 示例超过深度或字段数量限制")
		}
		s := JSONShape{}
		switch v := value.(type) {
		case map[string]any:
			s.Type = "object"
			s.Properties = map[string]JSONShape{}
			for key, item := range v {
				child, err := infer(item, depth+1)
				if err != nil {
					return s, err
				}
				s.Properties[key] = child
			}
		case []any:
			s.Type = "array"
			for _, item := range v {
				child, err := infer(item, depth+1)
				if err != nil {
					return s, err
				}
				if s.Items == nil {
					s.Items = &child
				} else if s.Items.Type != child.Type {
					return s, errors.New("示例数组元素类型须一致")
				}
			}
		case string:
			s.Type = "string"
		case float64:
			s.Type = "number"
		case bool:
			s.Type = "boolean"
		case nil:
			s.Type = "null"
		}
		return s, nil
	}
	s, err := infer(value, 0)
	return value, s, err
}

// JSONValue reads an RFC 6901 pointer. Empty pointer denotes the document root.
func JSONValue(value any, pointer string) (any, error) {
	if pointer == "" {
		return value, nil
	}
	if !ValidJSONPointer(pointer) {
		return nil, errors.New("JSON 字段路径无效")
	}
	for _, part := range strings.Split(pointer[1:], "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch v := value.(type) {
		case map[string]any:
			next, exists := v[part]
			if !exists {
				return nil, fmt.Errorf("字段 %s 不存在", pointer)
			}
			value = next
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(v) || strconv.Itoa(i) != part {
				return nil, fmt.Errorf("数组索引 %s 无效", pointer)
			}
			value = v[i]
		default:
			return nil, fmt.Errorf("字段 %s 不存在", pointer)
		}
	}
	return value, nil
}

// ReplaceJSONValue changes an existing value, never inventing a schema path.
func ReplaceJSONValue(root any, pointer string, value any) error {
	i := strings.LastIndex(pointer, "/")
	if i < 0 {
		return errors.New("请求字段须指向 JSON 值")
	}
	parent, err := JSONValue(root, pointer[:i])
	if err != nil {
		return err
	}
	key := strings.ReplaceAll(strings.ReplaceAll(pointer[i+1:], "~1", "/"), "~0", "~")
	switch v := parent.(type) {
	case map[string]any:
		if _, ok := v[key]; !ok {
			return errors.New("映射字段不存在")
		}
		v[key] = value
	case []any:
		index, err := strconv.Atoi(key)
		if err != nil || index < 0 || index >= len(v) {
			return errors.New("映射索引无效")
		}
		v[index] = value
	default:
		return errors.New("映射字段父节点不可写入")
	}
	return nil
}

func (c JSONContract) Compile() (JSONContract, error) {
	root, shape, err := ParseJSONExample(c.Example)
	if err != nil {
		return c, err
	}
	if len(c.Bindings) > 64 {
		return c, errors.New("字段映射不能超过 64 项")
	}
	for name, pointer := range c.Bindings {
		if pointer == "" {
			return c, fmt.Errorf("%s 映射不能为空", name)
		}
		if _, err := JSONValue(root, pointer); err != nil {
			return c, err
		}
	}
	c.Schema = &shape
	return c, nil
}

// RequireStringBindings rejects unusable result/status mappings at configuration
// time rather than accepting a demo that the runtime adapter cannot extract.
func (c JSONContract) RequireStringBindings(names ...string) error {
	root, _, err := ParseJSONExample(c.Example)
	if err != nil {
		return err
	}
	for _, name := range names {
		pointer := c.Bindings[name]
		if pointer == "" {
			return fmt.Errorf("请映射 %s 字段", name)
		}
		value, err := JSONValue(root, pointer)
		if err != nil {
			return err
		}
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s 示例字段必须是字符串", name)
		}
	}
	return nil
}

// ValidateMappings checks only mapped fields: vendors can add unrelated fields
// and omit optional example metadata without breaking a configured contract.
func (c JSONContract) ValidateMappings(value any, optional ...string) error {
	if c.Schema == nil {
		return errors.New("响应 Schema 尚未编译")
	}
	for name, pointer := range c.Bindings {
		skip := false
		for _, field := range optional {
			if name == field {
				skip = true
			}
		}
		if skip {
			continue
		}
		actual, err := JSONValue(value, pointer)
		if err != nil {
			return err
		}
		shape := *c.Schema
		for _, part := range strings.Split(pointer[1:], "/") {
			part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
			if shape.Type == "array" && shape.Items != nil {
				shape = *shape.Items
			} else {
				child, ok := shape.Properties[part]
				if !ok {
					return errors.New("Schema 缺少映射字段")
				}
				shape = child
			}
		}
		switch shape.Type {
		case "string":
			if _, ok := actual.(string); !ok {
				return fmt.Errorf("字段 %s 应为字符串", pointer)
			}
		case "number":
			if _, ok := actual.(float64); !ok {
				return fmt.Errorf("字段 %s 应为数值", pointer)
			}
		case "boolean":
			if _, ok := actual.(bool); !ok {
				return fmt.Errorf("字段 %s 应为布尔值", pointer)
			}
		case "array":
			if _, ok := actual.([]any); !ok {
				return fmt.Errorf("字段 %s 应为数组", pointer)
			}
		case "object":
			if _, ok := actual.(map[string]any); !ok {
				return fmt.Errorf("字段 %s 应为对象", pointer)
			}
		}
	}
	return nil
}
