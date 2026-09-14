package music

import (
	"net/http"
	"net/http/httptest"
	"testing"

	musicmodel "github.com/guohuiyuan/music-lib/model"
	"github.com/hezebin/media-dl/internal/httpx"
)

func TestValidateSearchResultsKeepsInvalidAndAddsCandidates(t *testing.T) {
	const probeCookie = "MUSIC_U=test-cookie"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/invalid" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.Header.Get("Cookie") != probeCookie {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusPartialContent)
	}))
	defer server.Close()

	provider := Provider{
		Name: "netease",
		GetDownloadURL: func(song *musicmodel.Song) (string, error) {
			return song.URL, nil
		},
	}
	songs := []musicmodel.Song{
		{ID: "invalid", URL: server.URL + "/invalid"},
		{ID: "valid", URL: server.URL + "/valid"},
		{ID: "candidate", URL: server.URL + "/candidate"},
	}

	result, err := New("").validateSearchResults(provider, songs, 2, probeCookie)
	if err != nil {
		t.Fatalf("validateSearchResults() error = %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("validateSearchResults() returned %d songs, want 3", len(result))
	}
	if !result[0].IsInvalid || result[1].IsInvalid || result[2].IsInvalid {
		t.Fatalf("unexpected validity flags: invalid=%v,%v,%v", result[0].IsInvalid, result[1].IsInvalid, result[2].IsInvalid)
	}
}

func TestProbeDownloadURLFallsBackWithoutCookieAndRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := httpx.New(httpx.Options{})
	if err != nil {
		t.Fatalf("httpx.New() error = %v", err)
	}
	status, err := probeDownloadURL(client, server.URL, "MUSIC_U=test-cookie", "netease")
	if err != nil || status != http.StatusOK {
		t.Fatalf("probeDownloadURL() = (%d, %v), want (%d, nil)", status, err, http.StatusOK)
	}
}
