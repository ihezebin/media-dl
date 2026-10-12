package httpserver

import (
	"net/url"
	"testing"

	musicmodel "github.com/guohuiyuan/music-lib/model"
)

func TestUnproxyURL(t *testing.T) {
	original := "https://cdn.example.com/audio/song.mp3?token=a+b&quality=lossless#auth=abc"
	proxied := proxyPath + "?url=" + url.QueryEscape(original)
	if got := unproxyURL(proxied); got != original {
		t.Fatalf("unproxyURL(%q) = %q, want %q", proxied, got, original)
	}
	legacyProxied := legacyProxyPath + "?url=" + url.QueryEscape(original)
	if got := unproxyURL(legacyProxied); got != original {
		t.Fatalf("unproxyURL(%q) = %q, want %q", legacyProxied, got, original)
	}
	if got := unproxyURL(original); got != original {
		t.Fatalf("unproxyURL changed original URL to %q", got)
	}
}

func TestDefaultMusicProxyHeadersBilibili(t *testing.T) {
	for _, raw := range []string{
		"https://cn-jsnt-ct-01-07.bilivideo.com/upgcxcode/video.mp4",
		"https://xy39x184x180x117xy.mcdn.bilivideo.cn:8082/v1/resource/audio.m4s",
	} {
		target, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		headers := defaultMusicProxyHeaders(target)
		if headers["Referer"] != "https://www.bilibili.com/" || headers["Origin"] != "https://www.bilibili.com" {
			t.Fatalf("defaultMusicProxyHeaders(%q) = %#v", raw, headers)
		}
	}
}

func TestUnproxyMusicSong(t *testing.T) {
	originalURL := "https://cdn.example.com/song.mp3"
	originalCover := "https://img.example.com/cover.jpg"
	originalLink := "https://music.example.com/song/1"
	song := &musicmodel.Song{
		URL:   proxyPath + "?url=" + url.QueryEscape(originalURL),
		Cover: proxyPath + "?url=" + url.QueryEscape(originalCover),
		Link:  proxyPath + "?url=" + url.QueryEscape(originalLink),
		Extra: map[string]string{"lyric_url": proxyPath + "?url=" + url.QueryEscape("https://lyrics.example.com/song.lrc"), "id": "1"},
	}
	unproxyMusicSong(song)
	if song.URL != originalURL || song.Cover != originalCover || song.Link != originalLink {
		t.Fatalf("song URLs were not restored: %+v", song)
	}
	if song.Extra["lyric_url"] != "https://lyrics.example.com/song.lrc" || song.Extra["id"] != "1" {
		t.Fatalf("song extra fields were not restored: %+v", song.Extra)
	}
}

func TestParseProxyTarget(t *testing.T) {
	if _, err := parseProxyTarget("https://cdn.example.com/song.mp3"); err != nil {
		t.Fatalf("parseProxyTarget rejected HTTPS URL: %v", err)
	}
	for _, raw := range []string{
		"",
		"file:///tmp/song.mp3",
		"//cdn.example.com/song.mp3",
		"http://127.0.0.1:8080/internal",
		"http://192.168.1.10/internal",
	} {
		if _, err := parseProxyTarget(raw); err == nil {
			t.Fatalf("parseProxyTarget(%q) unexpectedly succeeded", raw)
		}
	}
}
