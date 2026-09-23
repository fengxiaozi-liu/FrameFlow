package provider

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type resultTransport func(*http.Request) (*http.Response, error)

func (f resultTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestResultStorePersistsRemoteAssetAndRejectsUntrustedHosts(t *testing.T) {
	dir := t.TempDir()
	store := ResultStore{Directory: dir, HTTP: &http.Client{Transport: resultTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "results.aliyuncs.com" {
			t.Fatal(req.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("image")), Header: make(http.Header)}, nil
	})}}
	if _, err := store.Save(context.Background(), "task-1", task.KindImage, "https://127.0.0.1/image.png"); err == nil {
		t.Fatal("local result URL was accepted")
	}
	path, err := store.Save(context.Background(), "20260914160358.943272300", task.KindImage, "https://results.aliyuncs.com/image.png")
	if err != nil || path != "/media/generated-20260914160358.943272300.png" {
		t.Fatal(path, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "generated-20260914160358.943272300.png"))
	if err != nil || string(data) != "image" {
		t.Fatal(string(data), err)
	}
}

func TestVideoResultMustBeValidMP4BeforePersistence(t *testing.T) {
	dir := t.TempDir()
	body := "not a video"
	store := ResultStore{Directory: dir, HTTP: &http.Client{Transport: resultTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	if _, err := store.Save(context.Background(), "video-1", task.KindVideo, "https://results.aliyuncs.com/video.mp4"); err == nil {
		t.Fatal("invalid MP4 accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "generated-video-1.mp4")); !os.IsNotExist(err) {
		t.Fatal("invalid result persisted", err)
	}
	body = "\x00\x00\x00\x18ftypisom" + strings.Repeat("\x00", 16)
	saved, err := store.Save(context.Background(), "video-1", task.KindVideo, "https://results.aliyuncs.com/video.mp4")
	if err != nil || saved != "/media/generated-video-1.mp4" {
		t.Fatal(saved, err)
	}
}
