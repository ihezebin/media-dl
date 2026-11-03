package music

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestValidationPreservesAccessRestrictionReason(t *testing.T) {
	provider := Provider{Name: "test", GetDownloadURL: func(song *musicmodel.Song) (string, error) {
		return "", errors.New("请先登录，VIP 歌曲仅提供试听片段")
	}}
	songs, err := New("").validateSearchResults(provider, []musicmodel.Song{{ID: "restricted"}}, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if !songs[0].IsInvalid || !strings.Contains(songs[0].Extra["invalid_reason"], "请先登录") || !strings.Contains(songs[0].Extra["invalid_reason"], "试听片段") {
		t.Fatal("access reason was lost")
	}
}

func TestPlayableURLStopsBeforeFallbackWhenPrimaryWorks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusPartialContent) }))
	defer server.Close()
	client, _ := httpx.New(httpx.Options{})
	provider := Provider{Name: "kugou", GetDownloadURL: func(song *musicmodel.Song) (string, error) { return server.URL, nil }, GetDownloadURLFallback: func(song *musicmodel.Song) (string, error) {
		t.Error("unnecessary fallback")
		return "", errors.New("failed")
	}}
	if _, err := New("").resolvePlayableURL(provider, &musicmodel.Song{}, client, ""); err != nil {
		t.Fatal(err)
	}
}

func TestKugouChallengeDoesNotImplyPaymentRequired(t *testing.T) {
	err := musicResolveError([]string{"酷狗网页需要安全验证，请打开酷狗官网完成验证后重试", "酷狗：需要付费"})
	if !strings.Contains(err.Error(), "安全验证") || strings.Contains(err.Error(), "付费") {
		t.Fatal(err)
	}
}

func TestKugouCachedPreviewIsReResolved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/preview" {
			t.Error("cached preview should not be reused")
		}
		w.WriteHeader(http.StatusPartialContent)
	}))
	defer server.Close()
	client, _ := httpx.New(httpx.Options{})
	song := &musicmodel.Song{URL: server.URL + "/preview", Extra: map[string]string{"is_trial": "1"}}
	called := false
	provider := Provider{Name: "kugou", GetDownloadURL: func(song *musicmodel.Song) (string, error) {
		called = true
		delete(song.Extra, "is_trial")
		return server.URL + "/full", nil
	}}
	got, err := New("").resolvePlayableURL(provider, song, client, "")
	if err != nil || !called || got != server.URL+"/full" {
		t.Fatal("preview was not refreshed", err)
	}
}
