package provider

import (
	"errors"
	"fmt"
	"slices"
)

const (
	TextToImage  = "text_to_image"
	ImageToImage = "image_to_image"
)

// ResolvedImageBinding translates pre-mode configurations without using model IDs.
// A previously configured reference limit represented reference-image capability.
func (b OperationBinding) ResolvedImageBinding() OperationBinding {
	if b.ImageModes == nil {
		b.ImageModes = []string{TextToImage}
		if b.MaxReferences > 0 {
			b.ImageModes = append(b.ImageModes, ImageToImage)
			b.MinReferences = 1
			if b.MaxImageBytes == 0 {
				b.MaxImageBytes = MaxLocalImageBytes
			}
		}
	}
	return b
}

func (b OperationBinding) SupportsImageMode(mode string) bool {
	return slices.Contains(b.ResolvedImageBinding().ImageModes, mode)
}

func (b OperationBinding) validateImageModes() error {
	b = b.ResolvedImageBinding()
	if len(b.ImageModes) == 0 {
		return errors.New("请至少启用文生图或图生图一种能力")
	}
	for i, mode := range b.ImageModes {
		if (mode != TextToImage && mode != ImageToImage) || slices.Contains(b.ImageModes[:i], mode) {
			return errors.New("图片生成模式无效或重复")
		}
	}
	if b.SupportsImageMode(ImageToImage) {
		if b.MinReferences < 1 || b.MaxReferences < b.MinReferences || b.MaxReferences > 100 {
			return errors.New("图生图需配置有效参考图数量范围（1 至 100 张）")
		}
		if b.MaxImageBytes <= 0 || b.MaxImageBytes > MaxLocalImageBytes {
			return errors.New("图生图需配置单张图片上限，当前本地传输最多 20 MB")
		}
	}
	return nil
}

// ValidateImageInput is used both at submission and by protocol adapters.
// Omitted mode is reserved for legacy callers; references determine their intent.
func (b OperationBinding) ValidateImageInput(mode string, count int) error {
	b = b.ResolvedImageBinding()
	if err := b.validateImageModes(); err != nil {
		return err
	}
	if mode == "" {
		mode = TextToImage
		if count > 0 {
			mode = ImageToImage
		}
	}
	if !b.SupportsImageMode(mode) {
		return errors.New("该模型未启用此图片生成模式，请先在模型的协议与参数中配置")
	}
	if mode == TextToImage && count != 0 {
		return errors.New("文生图不能携带参考图片，请选择图生图")
	}
	if mode == ImageToImage && (count < b.MinReferences || count > b.MaxReferences) {
		return fmt.Errorf("图生图需要 %d–%d 张参考图片", b.MinReferences, b.MaxReferences)
	}
	return nil
}
