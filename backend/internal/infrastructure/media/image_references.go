package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

// ImageReferences implements the submission and delivery ports for local editing inputs.
type ImageReferences struct{ Directory string }

func (s ImageReferences) SnapshotReference(ctx context.Context, asset material.Asset, label string, limit int64) (task.MediaReference, error) {
	data, _, err := s.read(ctx, asset.URL, limit)
	if err != nil {
		return task.MediaReference{}, err
	}
	hash := sha256.Sum256(data)
	return task.MediaReference{MaterialID: asset.ID, URL: asset.URL, Label: label, ContentHash: hex.EncodeToString(hash[:])}, nil
}

func (s ImageReferences) ReferenceImageData(ctx context.Context, ref task.MediaReference, limit int64) (string, error) {
	data, mime, err := s.read(ctx, ref.URL, limit)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	if ref.ContentHash == "" || hex.EncodeToString(hash[:]) != ref.ContentHash {
		return "", errors.New("reference content changed since submission")
	}
	// Validate and encode the same bytes; never reread after verifying the digest.
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func (s ImageReferences) read(ctx context.Context, url string, limit int64) ([]byte, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	name := strings.TrimPrefix(url, "/media/")
	if s.Directory == "" || name == "" || name == url || filepath.Base(name) != name || strings.ContainsAny(name, `/\`) {
		return nil, "", errors.New("请选择已上传到当前项目或素材库的本地图片")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return nil, "", errors.New("图生图参考图片必须为 PNG 或 JPEG")
	}
	file, err := os.Open(filepath.Join(s.Directory, name))
	if err != nil {
		return nil, "", errors.New("参考图片文件无法读取，请重新上传")
	}
	defer file.Close()
	if limit <= 0 || limit > provider.MaxLocalImageBytes {
		return nil, "", errors.New("图片大小上限未配置或超过本地传输限制")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 || int64(len(data)) > limit {
		return nil, "", fmt.Errorf("参考图片不能为空，且不得超过模型限制 %d 字节", limit)
	}
	mime := http.DetectContentType(data)
	if mime != "image/png" && mime != "image/jpeg" {
		return nil, "", errors.New("图生图参考图片必须为 PNG 或 JPEG")
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		return nil, "", errors.New("参考图片内容无效")
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	return data, mime, nil
}
