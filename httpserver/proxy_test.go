package httpserver

import (
	"net/url"
	"testing"

	musicmodel "github.com/guohuiyuan/music-lib/model"
	videomodel "github.com/hezebin/media-dl/internal/video/model"
)

func TestProxyURLRoundTrip(t *testing.T) {
	original := "https://cdn.example.com/audio/song.mp3?token=a+b&quality=lossless#auth=abc"
	proxied := proxyURL(original)
	if proxied == original || !isProxyURL(proxied) {
		t.Fatalf("proxyURL(%q) = %q", original, proxied)
	}
	if got := unproxyURL(proxied); got != original {
		t.Fatalf("unproxyURL(%q) = %q, want %q", proxied, got, original)
	}
	if got := proxyURL(proxied); got != proxied {
		t.Fatalf("proxyURL double-wrapped %q as %q", proxied, got)
	}
}

func TestProxyMusicSong(t *testing.T) {
	song := &musicmodel.Song{
		URL:   "https://cdn.example.com/song.mp3",
		Cover: "https://img.example.com/cover.jpg",
		Link:  "https://music.example.com/song/1",
		Extra: map[string]string{"lyric_url": "https://lyrics.example.com/song.lrc", "id": "1"},
	}
	proxyMusicSong(song)
	for name, got := range map[string]string{
		"url":         song.URL,
		"cover":       song.Cover,
		"link":        song.Link,
		"lyric_url":   song.Extra["lyric_url"],
		"plain extra": song.Extra["id"],
	} {
		if name == "plain extra" {
			if got != "1" {
				t.Fatalf("extra id = %q, want 1", got)
			}
			continue
		}
		parsed, err := url.Parse(got)
		if err != nil || parsed.Path != proxyPath {
			t.Fatalf("song %s = %q, want proxy URL", name, got)
		}
	}
	unproxyMusicSong(song)
	if song.URL != "https://cdn.example.com/song.mp3" || song.Cover != "https://img.example.com/cover.jpg" || song.Link != "https://music.example.com/song/1" {
		t.Fatalf("song URLs were not restored: %+v", song)
	}
	if song.Extra["lyric_url"] != "https://lyrics.example.com/song.lrc" {
		t.Fatalf("lyric URL was not restored: %q", song.Extra["lyric_url"])
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

func TestProxyVideoInfo(t *testing.T) {
	info := &videomodel.VideoInfo{
		VideoURL:   "https://cdn.example.com/video.mp4",
		CoverURL:   "https://img.example.com/cover.jpg",
		WebpageURL: "https://video.example.com/watch/1",
		Formats: []videomodel.Format{{
			URL:      "https://cdn.example.com/video-1080.mp4",
			AudioURL: "https://cdn.example.com/audio.m4a",
			Headers: map[string]string{
				"Referer":    "https://www.douyin.com/",
				"Origin":     "https://www.douyin.com",
				"User-Agent": "Mozilla/5.0 test",
			},
		}},
	}
	proxyVideoInfo(info)
	for name, value := range map[string]string{
		"video":  info.VideoURL,
		"cover":  info.CoverURL,
		"format": info.Formats[0].URL,
		"audio":  info.Formats[0].AudioURL,
	} {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Path != proxyPath {
			t.Fatalf("%s URL = %q, want proxy URL", name, value)
		}
		if got := parsed.Query().Get("referer"); got != "https://www.douyin.com/" {
			t.Fatalf("%s referer = %q, want Douyin referer", name, got)
		}
	}
	if info.WebpageURL != "https://video.example.com/watch/1" {
		t.Fatalf("webpage URL was changed: %q", info.WebpageURL)
	}
	unproxyVideoInfo(info)
	if info.VideoURL != "https://cdn.example.com/video.mp4" || info.CoverURL != "https://img.example.com/cover.jpg" || info.Formats[0].URL != "https://cdn.example.com/video-1080.mp4" || info.Formats[0].AudioURL != "https://cdn.example.com/audio.m4a" {
		t.Fatalf("video URLs were not restored: %+v", info)
	}
}
