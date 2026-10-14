package provider

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxRatioOptions = 64
const MaxRatioLength = 64

// Ratio values are provider parameters, including named modes such as adaptive.
// Keep their case and spelling; a model configuration does not expand a protocol.
func ValidateRatioValue(value string) error {
	if value == "" || utf8.RuneCountInString(value) > MaxRatioLength ||
		strings.ContainsAny(value, ",，") || strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) >= 0 {
		return errors.New("画面比例须为 1–64 个字符，不能包含空白、控制字符或逗号")
	}
	return nil
}

func (b OperationBinding) validateRatios() error {
	if len(b.Ratios) == 0 || len(b.Ratios) > MaxRatioOptions {
		return errors.New("请配置 1–64 个画面比例选项")
	}
	seen := make(map[string]bool, len(b.Ratios))
	for _, value := range b.Ratios {
		if err := ValidateRatioValue(value); err != nil {
			return err
		}
		if seen[value] {
			return fmt.Errorf("画面比例选项重复：%s", value)
		}
		seen[value] = true
		if b.Protocol != CustomProtocol && !slices.Contains([]string{"16:9", "9:16", "1:1"}, value) {
			return fmt.Errorf("当前内置协议不支持比例 %s；自定义参数请配置相应的自定义 JSON 协议", value)
		}
	}
	return nil
}

func (b OperationBinding) ValidateAspectRatio(value string) error {
	if err := ValidateRatioValue(value); err != nil {
		return err
	}
	if !slices.Contains(b.Ratios, value) {
		return errors.New("当前画面比例不在该模型配置范围内，请重新选择")
	}
	return nil
}
