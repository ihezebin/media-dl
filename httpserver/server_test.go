package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hezebin/media-dl/internal/music"
)

func TestServerRoutes(t *testing.T) {
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<!doctype html><title>media-dl</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	server, err := New(context.Background(), Config{WebDir: webDir, OutputDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want int
	}{
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
		server.captchaStore.mu.Lock()
		server.captchaStore.tokens[token] = time.Now().Add(time.Minute)
		server.captchaStore.mu.Unlock()

		req := httptest.NewRequest(http.MethodPost, "/api/music/search/verified", nil)
		req.Header.Set("X-Captcha-Token", token)
		resp := httptest.NewRecorder()
		server.app.Engine().ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d after captcha header was accepted; body = %s", resp.Code, http.StatusBadRequest, resp.Body.String())
		}
	})
}
