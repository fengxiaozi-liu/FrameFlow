package bailian

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

func TestConfiguredAudioUploadReturnsExpiringSignedURL(t *testing.T) {
	t.Setenv("FRAMEFLOW_AUDIO_OBJECT_BASE_URL", "https://examplebucket.oss-cn-beijing.aliyuncs.com")
	t.Setenv("FRAMEFLOW_AUDIO_OBJECT_BUCKET", "examplebucket")
	t.Setenv("FRAMEFLOW_AUDIO_OBJECT_ACCESS_KEY_ID", "test-id")
	t.Setenv("FRAMEFLOW_AUDIO_OBJECT_ACCESS_KEY_SECRET", "test-secret")
	local := filepath.Join(t.TempDir(), "voice.mp3")
	if err := os.WriteFile(local, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	uploaded := false
	client := Client{HTTP: &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		uploaded = true
		if req.Method != http.MethodPut || !strings.HasPrefix(req.Header.Get("Authorization"), "OSS test-id:") || req.Header.Get("Content-Type") != "audio/mpeg" || !strings.HasPrefix(req.URL.Path, "/frameflow/audio/") {
			t.Fatalf("unexpected OSS upload: %s %s %s", req.Method, req.URL, req.Header.Get("Authorization"))
		}
		_, _ = io.Copy(io.Discard, req.Body)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}}
	result, err := client.UploadAudio(context.Background(), provider.Connection{}, provider.Model{}, local)
	if err != nil {
		t.Fatal(err)
	}
	if !uploaded || !strings.Contains(result, "OSSAccessKeyId=test-id") || !strings.Contains(result, "Expires=") || !strings.Contains(result, "Signature=") {
		t.Fatal(result)
	}
}
