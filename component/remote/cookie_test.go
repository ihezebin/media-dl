package remote

import (
	"net/url"
	"testing"

	"github.com/hezebin/media-dl/internal/httpx"
)

func TestApplyCookieHeader(t *testing.T) {
	client, err := httpx.New(httpx.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ApplyCookieHeader(client, "UIFID=uifid-test; ttwid=ttwid-test")
	for _, rawURL := range []string{"https://www.douyin.com/video/1", "https://www.youtube.com/watch?v=test"} {
		u, err := url.Parse(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		cookies := client.HTTP().Jar.Cookies(u)
		if len(cookies) != 2 {
			t.Fatalf("cookies for %s = %d, want 2", rawURL, len(cookies))
		}
	}
}

func TestCookieHeaderFromTextForPlatformFiltersDomains(t *testing.T) {
	raw := "# Netscape HTTP Cookie File\n" +
		".kugou.com\tTRUE\t/\tFALSE\t0\tkg_mid\tkugou-value\n" +
		".music.163.com\tTRUE\t/\tFALSE\t0\tMUSIC_U\tnetease-value\n" +
		"atc.bmw-brilliance.cn\tFALSE\t/\tFALSE\t0\tSESSION\tunrelated\n"
	got, err := CookieHeaderFromTextForPlatform(raw, "kugou")
	if err != nil {
		t.Fatal(err)
	}
	if got != "kg_mid=kugou-value" {
		t.Fatalf("filtered cookie header = %q, want only Kugou cookie", got)
	}
}

func TestCookieHeaderFromTextForPlatformKeepsPlainHeader(t *testing.T) {
	got, err := CookieHeaderFromTextForPlatform("kg_mid=kugou-value; MUSIC_U=netease-value", "kugou")
	if err != nil {
		t.Fatal(err)
	}
	if got != "kg_mid=kugou-value; MUSIC_U=netease-value" {
		t.Fatalf("plain cookie header = %q", got)
	}
}
