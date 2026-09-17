package web

import (
	"net/url"
	"testing"

	"github.com/hezebin/media-dl/internal/video/model"
)

func TestPlatformMatches(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"youtube", "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
		{"tiktok", "https://www.tiktok.com/@_halima_07_/video/7492784957073493256"},
		{"kuaishou", "https://www.kuaishou.com/short-video/3xegqfwigw73xns"},
		{"baidu", "https://haokan.baidu.com/v?vid=4851961422851197974"},
		{"twitter", "https://x.com/example/status/1234567890"},
		{"douyu", "https://www.douyu.com/5720533"},
		{"huya", "https://www.huya.com/lpl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, p := range platforms {
				if p.name == tc.name && !p.match(tc.url) {
					t.Fatalf("%s did not match %s", tc.name, tc.url)
				}
			}
		})
	}
}

func TestIDs(t *testing.T) {
	if got := youtubeID("https://youtu.be/dQw4w9WgXcQ"); got != "dQw4w9WgXcQ" {
		t.Fatalf("youtube id = %q", got)
	}
	if got := twitterID("https://x.com/example/status/1234567890"); got != "1234567890" {
		t.Fatalf("twitter id = %q", got)
	}
}

func TestTwitterSyndicationURLIncludesToken(t *testing.T) {
	parsed, err := url.Parse(twitterSyndicationURL("2100149744349942038"))
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.Query().Get("token"); got != "0" {
		t.Fatalf("token = %q, want 0", got)
	}
}

func TestCollectPageFormats(t *testing.T) {
	body := `<meta property="og:video" content="https://cdn.example.test/video.mp4"><script type="application/ld+json">{"contentUrl":"https://cdn.example.test/second.mp4"}</script>`
	formats := collectPageFormats(body, map[string]string{"Referer": "https://example.test/"})
	if len(formats) != 2 {
		t.Fatalf("formats = %d, want 2", len(formats))
	}
	if formats[0].URL == "" || formats[0].Ext != "mp4" || !formats[0].HasVideo {
		t.Fatalf("unexpected format: %+v", formats[0])
	}
}

func TestYouTubeFormatsRecognizeMuxedStreams(t *testing.T) {
	data := map[string]any{"streamingData": map[string]any{"formats": []any{
		map[string]any{"itag": "18", "url": "https://video.example.test/18.mp4", "mimeType": `video/mp4; codecs="avc1.42001E, mp4a.40.2"`, "qualityLabel": "360p"},
		map[string]any{"itag": "313", "url": "https://video.example.test/313.webm", "mimeType": `video/webm; codecs="vp9"`, "qualityLabel": "2160p"},
	}}}
	formats := youtubeFormats(data, nil)
	if len(formats) != 2 {
		t.Fatalf("formats = %d, want 2", len(formats))
	}
	if best := (&model.VideoInfo{Formats: formats}).BestFormat(); best == nil || best.FormatID != "18" || !best.HasAudio {
		t.Fatalf("best format = %+v, want muxed format 18", best)
	}
}

func TestCollectPageFormatsDoesNotSelectTikTokMusicAsVideo(t *testing.T) {
	body := `<script type="application/ld+json">{"music":{"playUrl":"https://cdn.example.test/audio.mp3?mime_type=audio_mpeg"},"video":{"playAddr":"https://cdn.example.test/video.mp4?mime_type=video_mp4"}}</script>`
	formats := collectPageFormats(body, nil)
	if len(formats) != 2 {
		t.Fatalf("formats = %d, want 2", len(formats))
	}
	var audio, video *model.Format
	for i := range formats {
		switch {
		case formats[i].URL == "https://cdn.example.test/audio.mp3?mime_type=audio_mpeg":
			audio = &formats[i]
		case formats[i].URL == "https://cdn.example.test/video.mp4?mime_type=video_mp4":
			video = &formats[i]
		}
	}
	if audio == nil || audio.HasVideo || !audio.HasAudio {
		t.Fatalf("audio format = %+v, want audio-only", audio)
	}
	if video == nil || !video.HasVideo || !video.HasAudio {
		t.Fatalf("video format = %+v, want muxed video", video)
	}
	if best := (&model.VideoInfo{Formats: formats}).BestFormat(); best != video {
		t.Fatalf("best format = %+v, want video format %+v", best, video)
	}
}
