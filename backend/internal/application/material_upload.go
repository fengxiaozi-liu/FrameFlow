package application

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
func (s MaterialService) Upload(g *gin.Context) {
	ctx := g.Request.Context()
	r := g.Request
	r.Body = http.MaxBytesReader(g.Writer, r.Body, 21<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(g, &Error{
			Kind:  "too_large",
			Code:  "upload_too_large",
			Cause: errors.New("upload must be 20 MB or smaller"),
		})
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(g, Invalid("file_required", err))
		return
	}
	defer file.Close()
	if err := ctx.Err(); err != nil {
		writeError(g, err)
		return
	}
	if header.Size > 20<<20 {
		writeError(g, &Error{
			Kind:  "too_large",
			Code:  "upload_too_large",
			Cause: errors.New("upload must be 20 MB or smaller"),
		})
		return
	}
	sample := make([]byte, 512)
	n, err := file.Read(sample)
	if err != nil && err != io.EOF {
		writeError(g, Invalid("invalid_media", err))
		return
	}
	contentType := http.DetectContentType(sample[:n])
	if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "audio/") {
		writeError(g, &Error{
			Kind:  "unsupported_media",
			Code:  "unsupported_media_type",
			Cause: errors.New("upload must be an image or audio file"),
		})
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(g, Invalid("invalid_media", err))
		return
	}
	name := filepath.Base(header.Filename)
	if name == "." || name == "" {
		writeError(g, Invalid("invalid_filename", errors.New("invalid filename")))
		return
	}
	id := time.Now().UTC().Format("20060102150405.000000000")
	asset, err := material.New(id, g.PostForm("name"), material.Kind(g.PostForm("kind")), time.Now().UTC())
	if err != nil {
		writeError(g, Invalid("invalid_material", err))
		return
	}
	asset.NormalizeLegacyKind()
	if !validMaterialKind(asset.Kind) || strings.HasPrefix(contentType, "image/") && asset.Kind != material.Scene && asset.Kind != material.Character && asset.Kind != material.Prop || strings.HasPrefix(contentType, "audio/") && asset.Kind != material.Voice && asset.Kind != material.Music {
		writeError(g, Invalid("invalid_material_kind", errors.New("category does not match media type")))
		return
	}
	if err := ctx.Err(); err != nil {
		writeError(g, err)
		return
	}
	if err := os.MkdirAll(s.UploadDir, 0750); err != nil {
		writeError(g, err)
		return
	}
	stored := id + filepath.Ext(name)
	path := filepath.Join(s.UploadDir, stored)
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
	if err != nil {
		writeError(g, err)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(path)
		}
	}()
	_, copyErr := io.Copy(out, contextReader{
		ctx:    ctx,
		reader: file,
	})
	closeErr := out.Close()
	if copyErr != nil {
		writeError(g, copyErr)
		return
	}
	if closeErr != nil {
		writeError(g, closeErr)
		return
	}
	asset.Media, err = inspectUploadedMedia(ctx, path, contentType)
	if err != nil {
		writeError(g, Invalid("invalid_media", err))
		return
	}
	asset.URL = "/media/" + stored
	if err := s.Repo.Save(ctx, asset); err != nil {
		writeError(g, err)
		return
	}
	committed = true
	g.JSON(http.StatusCreated, asset)
}
