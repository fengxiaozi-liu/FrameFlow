package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type ResultStore struct {
	Directory string
	HTTP      *http.Client
}

func (s ResultStore) Save(ctx context.Context, taskID string, kind task.Kind, remoteURL string) (string, error) {
	u, err := url.Parse(remoteURL)
	if err != nil || u.Scheme != "https" || !strings.HasSuffix(strings.ToLower(u.Hostname()), ".aliyuncs.com") {
		return "", errors.New("unexpected model result host")
	}
	for _, c := range taskID {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-' || c == '_') {
			return "", errors.New("invalid task id")
		}
	}
	if taskID == "" {
		return "", errors.New("missing task id")
	}
	ext, limit := ".png", int64(30<<20)
	if kind == task.KindVideo {
		ext, limit = ".mp4", 500<<20
	}
	if err := os.MkdirAll(s.Directory, 0750); err != nil {
		return "", err
	}
	name := "generated-" + taskID + ext
	client := s.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download result: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return "", errors.New("model result is too large")
	}
	file, err := os.CreateTemp(s.Directory, ".generated-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := io.Copy(file, io.LimitReader(resp.Body, limit+1)); err != nil {
		return "", err
	}
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if info.Size() > limit {
		return "", errors.New("model result is too large")
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(file.Name(), filepath.Join(s.Directory, name)); err != nil {
		return "", err
	}
	return "/media/" + name, nil
}
