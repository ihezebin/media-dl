package music

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	musicmodel "github.com/guohuiyuan/music-lib/model"
	"github.com/hezebin/media-dl/internal/httpx"
)

func TestNeteaseDownloadUsesResolvedFormatAndHeaders(t *testing.T) {
	const cookie = "MUSIC_U=format-regression-fixture"
	audio := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "http://music.163.com/" || r.Header.Get("Origin") != "http://music.163.com" || r.Header.Get("Cookie") != cookie {
			t.Error("Netease media headers or account cookie lost")
		}
		_, _ = io.WriteString(w, "fLaC-fixture")
	}))
	defer audio.Close()
	client, err := httpx.New(httpx.Options{})
	if err != nil {
		t.Fatal(err)
	}
	previous := http.DefaultTransport
	http.DefaultTransport = kugouTestTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"code":200,"profile":{"vipType":11}}`
		if strings.Contains(r.URL.Path, "/player/url") {
			body = `{"data":[{"url":"` + audio.URL + `/audio.flac","type":"flac","code":200}]}`
		} else if !strings.Contains(r.URL.Path, "/account/get") {
			t.Errorf("unexpected upstream path: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = previous })
	service := New(cookie)
	song := &musicmodel.Song{ID: "100001", Source: "netease", Name: "fixture", Ext: "mp3"}
	dir := t.TempDir()
	result, err := service.Download(client, song, DownloadOptions{OutputDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if result.AudioPath != filepath.Join(dir, "fixture.flac") {
		t.Fatalf("wrong format: %s", result.AudioPath)
	}
	body, err := os.ReadFile(result.AudioPath)
	if err != nil || string(body) != "fLaC-fixture" {
		t.Fatalf("download changed: %q, %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "fixture.mp3")); !os.IsNotExist(err) {
		t.Fatal("stale MP3 output exists")
	}

	// Concurrent search probes must not share music-lib's mutable VIP pointer.
	provider, err := service.Provider("netease")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, id := range []string{"100002", "100003", "100004", "100005"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := provider.GetDownloadURL(&musicmodel.Song{ID: id, Source: "netease"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
