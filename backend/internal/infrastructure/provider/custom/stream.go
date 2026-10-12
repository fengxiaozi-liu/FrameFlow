package custom

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strings"

	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

// decodeStream implements bounded SSE/NDJSON framing independently of model
// identities. A truncated stream is never accepted as a complete generation.
func decodeStream(reader io.Reader, config domain.CustomInterface) (any, error) {
	limited := &io.LimitedReader{R: reader, N: (4 << 20) + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), 128<<10)
	var data []string
	var result any
	var accumulated strings.Builder
	done := false
	consume := func(payload string) error {
		if config.StreamDoneMarker != "" && payload == config.StreamDoneMarker {
			done = true
			return nil
		}
		if strings.TrimSpace(payload) == "" {
			return nil
		}
		var event any
		if json.Unmarshal([]byte(payload), &event) != nil {
			return errors.New("流式事件不是有效 JSON")
		}
		if config.StreamErrorPointer != "" {
			if value, err := domain.JSONValue(event, config.StreamErrorPointer); err == nil && value != nil && value != "" {
				return errors.New("远端流式生成失败")
			}
		}
		text, err := extract(event, config.ResultPointer)
		if err != nil {
			return nil
		} // heartbeats and metadata may have no result
		if config.StreamAppend {
			if accumulated.Len()+len(text) > 4<<20 {
				return errors.New("流式结果超过 4 MB")
			}
			accumulated.WriteString(text)
			if err := domain.ReplaceJSONValue(event, config.ResultPointer, accumulated.String()); err != nil {
				return err
			}
		}
		result = event
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if config.StreamMode == domain.NDJSONStream {
			if err := consume(line); err != nil {
				return nil, err
			}
		} else if line == "" {
			if err := consume(strings.Join(data, "\n")); err != nil {
				return nil, err
			}
			data = nil
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
		if done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.New("流式响应中断或事件过大")
	}
	if limited.N == 0 {
		return nil, errors.New("流式响应超过 4 MB")
	}
	if len(data) > 0 && !done {
		if err := consume(strings.Join(data, "\n")); err != nil {
			return nil, err
		}
	}
	if config.StreamDoneMarker != "" && !done {
		return nil, errors.New("流式响应未收到完成标记，请检查配置或连接")
	}
	if result == nil {
		return nil, errors.New("流式响应缺少已映射的结果字段")
	}
	return result, nil
}
