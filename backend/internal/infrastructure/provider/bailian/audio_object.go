package bailian

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

type audioObjectConfig struct {
	base                     *url.URL
	bucket, accessID, secret string
}

func configuredAudioObject() (audioObjectConfig, error) {
	raw := os.Getenv("FRAMEFLOW_AUDIO_OBJECT_BASE_URL")
	bucket := strings.TrimSpace(os.Getenv("FRAMEFLOW_AUDIO_OBJECT_BUCKET"))
	id := strings.TrimSpace(os.Getenv("FRAMEFLOW_AUDIO_OBJECT_ACCESS_KEY_ID"))
	secret := os.Getenv("FRAMEFLOW_AUDIO_OBJECT_ACCESS_KEY_SECRET")
	base, err := url.Parse(raw)
	if err != nil || base == nil || base.Scheme != "https" || base.Host == "" || bucket == "" || id == "" || secret == "" {
		return audioObjectConfig{}, errors.New("reference audio requires HTTPS OSS base URL, bucket and access credentials")
	}
	if strings.ContainsAny(bucket, "/\\") || base.RawQuery != "" || base.Path != "" && base.Path != "/" {
		return audioObjectConfig{}, errors.New("invalid audio object storage endpoint or bucket")
	}
	return audioObjectConfig{base: base, bucket: bucket, accessID: id, secret: secret}, nil
}

func (c audioObjectConfig) sign(value string) string {
	mac := hmac.New(sha1.New, []byte(c.secret))
	_, _ = mac.Write([]byte(value))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c Client) uploadConfiguredAudio(ctx context.Context, localPath string) (string, error) {
	config, err := configuredAudioObject()
	if err != nil {
		return "", err
	}
	file, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return "", err
	}
	if stat.Size() <= 0 || stat.Size() > 15<<20 {
		return "", errors.New("reference audio must be 1–15 MB")
	}
	ext := strings.ToLower(path.Ext(localPath))
	contentType := "audio/mpeg"
	if ext == ".wav" {
		contentType = "audio/wav"
	} else if ext != ".mp3" {
		return "", errors.New("reference audio must be MP3 or WAV")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	objectKey := "frameflow/audio/" + hex.EncodeToString(nonce[:]) + ext
	target := *config.base
	target.Path = "/" + objectKey
	resource := "/" + config.bucket + "/" + objectKey
	date := time.Now().UTC().Format(http.TimeFormat)
	signature := config.sign("PUT\n\n" + contentType + "\n" + date + "\n" + resource)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, target.String(), file)
	if err != nil {
		return "", err
	}
	request.ContentLength = stat.Size()
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Date", date)
	request.Header.Set("Authorization", "OSS "+config.accessID+":"+signature)
	response, err := c.httpClient().Do(request)
	if err != nil {
		return "", fmt.Errorf("audio OSS upload: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("audio OSS upload rejected (%d)", response.StatusCode)
	}
	expires := time.Now().Add(2 * time.Hour).Unix()
	query := url.Values{}
	query.Set("OSSAccessKeyId", config.accessID)
	query.Set("Expires", fmt.Sprint(expires))
	query.Set("Signature", config.sign("GET\n\n\n"+fmt.Sprint(expires)+"\n"+resource))
	target.RawQuery = query.Encode()
	return target.String(), nil
}
