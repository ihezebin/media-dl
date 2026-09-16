package web

import "testing"

func TestPlatformMatches(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"youtube", "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
		{"tiktok", "https://www.tiktok.com/@demo/video/1234567890123456789"},
		{"kuaishou", "https://www.kuaishou.com/short-video/abc123"},
		{"baidu", "https://haokan.baidu.com/v?vid=123"},
		{"twitter", "https://x.com/example/status/1234567890"},
		{"douyu", "https://www.douyu.com/123"},
		{"huya", "https://www.huya.com/123"},
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
