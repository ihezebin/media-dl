package music

import (
	"reflect"
	"testing"
)

func TestNormalizePlatform(t *testing.T) {
	tests := map[string]string{
		"网易云音乐":       "netease",
		"QQ音乐":        "qq",
		"酷狗":          "kugou",
		"Apple Music": "apple",
	}
	for input, want := range tests {
		got, err := NormalizePlatform(input)
		if err != nil {
			t.Fatalf("NormalizePlatform(%q) error = %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizePlatform(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSearchOptionsQuery(t *testing.T) {
	got := (SearchOptions{Artist: "周杰伦", Title: "晴天"}).Query()
	if got != "周杰伦 晴天" {
		t.Fatalf("Query() = %q, want %q", got, "周杰伦 晴天")
	}
}

func TestNormalizePlatforms(t *testing.T) {
	got, err := normalizePlatforms([]string{"qq, kugou", "QQ音乐"})
	if err != nil {
		t.Fatalf("normalizePlatforms() error = %v", err)
	}
	want := []string{"qq", "kugou"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizePlatforms() = %v, want %v", got, want)
	}
}

func TestProviders(t *testing.T) {
	service := New("")
	for _, name := range PlatformNames {
		provider, err := service.Provider(name)
		if err != nil {
			t.Fatalf("Provider(%q) error = %v", name, err)
		}
		if provider.Name != name || provider.Search == nil || provider.Parse == nil ||
			provider.GetDownloadURL == nil || provider.GetLyrics == nil {
			t.Fatalf("Provider(%q) returned incomplete provider: %+v", name, provider)
		}
	}
}

func TestAppleCookieValue(t *testing.T) {
	service := New("media-user-token=user-token; token=eyJheader.payload.signature")
	got, err := service.appleCookieValue()
	if err != nil {
		t.Fatalf("appleCookieValue() error = %v", err)
	}
	if got != service.cookie {
		t.Fatalf("appleCookieValue() = %q, want %q", got, service.cookie)
	}
}
