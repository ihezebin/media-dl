package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin/binding"
	"github.com/hezebin/media-dl/application/dto"
	"github.com/hezebin/media-dl/component/cache"
	"github.com/hezebin/media-dl/internal/music"
)

func TestVideoCookieIsNotOverwrittenByRequestHeader(t *testing.T) {
	tests := []struct {
		name    string
		request any
	}{
		{name: "info", request: &dto.VideoInfoRequest{}},
		{name: "download", request: &dto.VideoDownloadRequest{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/video/info", bytes.NewBufferString(`{"cookie":"body-cookie"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Cookie", "browser-cookie=must-not-overwrite")
			if err := binding.JSON.Bind(req, tt.request); err != nil {
				t.Fatal(err)
			}
			if err := binding.Header.Bind(req, tt.request); err != nil {
				t.Fatal(err)
			}
			var got string
			switch value := tt.request.(type) {
			case *dto.VideoInfoRequest:
				got = value.Cookie
			case *dto.VideoDownloadRequest:
				got = value.Cookie
			}
			if got != "body-cookie" {
				t.Fatalf("cookie = %q, want body-cookie", got)
			}
		})
	}
}

func TestVideoRequestJSONRedactsCookie(t *testing.T) {
	for _, request := range []any{
		dto.VideoInfoRequest{URL: "https://example.com/video", Cookie: "secret-cookie"},
		dto.VideoDownloadRequest{URL: "https://example.com/video", Cookie: "secret-cookie"},
	} {
		encoded, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) == "" || bytes.Contains(encoded, []byte("secret-cookie")) {
			t.Fatalf("JSON leaked cookie: %s", encoded)
		}
		if !bytes.Contains(encoded, []byte(`[redacted]`)) {
			t.Fatalf("JSON did not contain redaction marker: %s", encoded)
		}
	}
}

func TestServerRoutes(t *testing.T) {
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<!doctype html><title>media-dl</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	server, err := New(context.Background(), Config{WebDir: webDir})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want int
	}{
		{name: "Stoplight", path: "/openapi", want: http.StatusOK},
		{name: "Redoc", path: "/redoc", want: http.StatusOK},
		{name: "RapiDoc", path: "/rapidoc", want: http.StatusOK},
		{name: "Swagger", path: "/swagger", want: http.StatusOK},
		{name: "metrics", path: "/metrics", want: http.StatusOK},
		{name: "pprof", path: "/debug/pprof/", want: http.StatusOK},
		{name: "music platforms", path: "/api/music/platforms", want: http.StatusOK},
		{name: "captcha", path: "/api/captcha", want: http.StatusOK},
		{name: "webui", path: "/", want: http.StatusOK},
		{name: "proxy missing url", path: "/api/proxy", want: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			resp := httptest.NewRecorder()
			server.app.Engine().ServeHTTP(resp, req)
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body = %s", resp.Code, tt.want, resp.Body.String())
			}
			if tt.name == "music platforms" {
				var body struct {
					Code int `json:"code"`
					Data struct {
						Platforms []string `json:"platforms"`
					} `json:"data"`
				}
				if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Code != 0 || len(body.Data.Platforms) != len(music.PlatformNames) {
					t.Fatalf("unexpected platform response: %s", resp.Body.String())
				}
			}
		})
	}
	t.Run("bad search", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/music/search", nil)
		resp := httptest.NewRecorder()
		server.app.Engine().ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body = %s", resp.Code, http.StatusBadRequest, resp.Body.String())
		}
	})
	t.Run("verified search requires captcha", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/music/search/verified", nil)
		resp := httptest.NewRecorder()
		server.app.Engine().ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d; body = %s", resp.Code, http.StatusUnauthorized, resp.Body.String())
		}
	})
	t.Run("verified search accepts only an issued header token", func(t *testing.T) {
		token := "test-captcha-token"
		server.Music.Service.Captcha = tokenCaptcha{token: token}

		req := httptest.NewRequest(http.MethodPost, "/api/music/search/verified", nil)
		req.Header.Set("X-Captcha-Token", token)
		resp := httptest.NewRecorder()
		server.app.Engine().ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d after captcha header was accepted; body = %s", resp.Code, http.StatusBadRequest, resp.Body.String())
		}
	})
}

func TestAPIRoutesDoNotRegisterWebUIOrCaptchaRoutes(t *testing.T) {
	server, err := New(context.Background(), Config{APIOnly: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		"/",
		"/api/captcha",
		"/api/captcha/verify",
		"/api/music/search/verified",
		"/api/video/info/verified",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			if path == "/api/captcha/verify" || path == "/api/music/search/verified" || path == "/api/video/info/verified" {
				req = httptest.NewRequest(http.MethodPost, path, nil)
			}
			resp := httptest.NewRecorder()
			server.app.Engine().ServeHTTP(resp, req)
			if resp.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d; body = %s", resp.Code, http.StatusNotFound, resp.Body.String())
			}
		})
	}

	t.Run("ordinary API remains available", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/music/platforms", nil)
		resp := httptest.NewRecorder()
		server.app.Engine().ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", resp.Code, http.StatusOK, resp.Body.String())
		}
	})
}

type tokenCaptcha struct{ token string }

func (t tokenCaptcha) Generate() (*cache.BehaviorCaptchaResponse, error) { return nil, nil }
func (t tokenCaptcha) Verify(cache.BehaviorCaptchaVerifyRequest) (string, time.Time, bool) {
	return "", time.Time{}, false
}
func (t tokenCaptcha) ValidToken(v string) bool { return v == t.token }
