package application

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"strings"
	"time"
)

func (a TaskResultApplier) saveReferenceImage(ctx context.Context, item task.Task) error {
	if a.Materials == nil {
		return errors.New("material repository required")
	}
	id := "result-" + item.ID
	if _, err := a.Materials.Get(ctx, id); err == nil {
		return nil
	} else if !errors.Is(err, fault.ErrNotFound) {
		return err
	}
	path, err := localMediaPath(a.Directory, item.ResultURL)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 || info.Size() > 10<<20 {
		return errors.New("参考图必须小于10MB")
	}
	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return errors.New("生成结果不是支持的图片")
	}
	if _, err = file.Seek(0, 0); err != nil {
		return err
	}
	sample := make([]byte, 512)
	n, err := file.Read(sample)
	if err != nil {
		return err
	}
	return a.Materials.Save(ctx, material.Asset{ID: id, Name: item.Input.MaterialName + " · 生成参考图", Kind: material.Kind(strings.TrimPrefix(item.Target, "reference_")), URL: item.ResultURL, CreatedAt: time.Now().UTC(), Tags: []string{"文生图"}, Media: material.MediaInfo{Width: cfg.Width, Height: cfg.Height, SizeBytes: info.Size(), Format: http.DetectContentType(sample[:n])}})
}
