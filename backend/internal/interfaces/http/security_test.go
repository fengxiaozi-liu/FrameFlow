package http

import (
	"bytes"
	"log"
	"mime/multipart"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	securityinfra "github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/security"
)

func TestRateLimit(t *testing.T) {
	server, done := testServer(t)
	defer done()
	server.RateLimit = 1
	handler := server.Routes()
	if got := request(t, handler, stdhttp.MethodGet, "/health", "").Code; got != stdhttp.StatusOK {
		t.Fatal(got)
	}
	if got := request(t, handler, stdhttp.MethodGet, "/health", "").Code; got != stdhttp.StatusTooManyRequests {
		t.Fatal(got)
	}
}

func TestCredentialIsEncryptedAndNeverReturnedOrLogged(t *testing.T) {
	server, done := testServer(t)
	defer done()
	vault, err := securityinfra.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server.Vault = vault
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previous)
	secret := "sk-secret-value"
	response := request(t, server.Routes(), stdhttp.MethodPost, "/api/providers", `{"code":"secure-story","name":"Secure","capability":"story","model":"mock-story","base_url":"mock://local","enabled":true,"status":"healthy","api_key":"`+secret+`"}`)
	if response.Code != stdhttp.StatusCreated {
		t.Fatal(response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), secret) || strings.Contains(logs.String(), secret) {
		t.Fatal("credential leaked through response or audit log")
	}
	if stored, ok := vault.Get("secure-story"); !ok || stored != secret {
		t.Fatal("credential was not stored in the encrypted vault")
	}
}

func TestUploadRejectsUnsupportedMediaAndOversizedBody(t *testing.T) {
	server, done := testServer(t)
	defer done()
	server.UploadDir = t.TempDir()
	handler := server.Routes()

	unsupported := multipartUpload(t, []byte("plain text"), "payload.txt")
	unsupportedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(unsupportedRecorder, unsupported)
	if unsupportedRecorder.Code != stdhttp.StatusUnsupportedMediaType {
		t.Fatal(unsupportedRecorder.Code, unsupportedRecorder.Body.String())
	}

	oversized := multipartUpload(t, bytes.Repeat([]byte{0xff}, (20<<20)+1), "large.jpg")
	oversizedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(oversizedRecorder, oversized)
	if oversizedRecorder.Code != stdhttp.StatusRequestEntityTooLarge {
		t.Fatal(oversizedRecorder.Code, oversizedRecorder.Body.String())
	}
}

func multipartUpload(t *testing.T, content []byte, name string) *stdhttp.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(content); err != nil {
		t.Fatal(err)
	}
	_ = writer.WriteField("name", "Upload")
	_ = writer.WriteField("kind", "visual")
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/materials/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}
