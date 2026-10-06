package downloader

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/hezebin/media-dl/internal/httpx"
)

func TestLoadCookiesFileIncludesHttpOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.txt")
	data := "# Netscape HTTP Cookie File\n#HttpOnly_.xiaohongshu.com\tTRUE\t/\tTRUE\t0\tweb_session\tsecret\n.xiaohongshu.com\tTRUE\t/\tFALSE\t0\ta1\tvalue\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cookies, err := LoadCookiesFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cookies) != 2 {
		t.Fatalf("got %d cookies, want 2", len(cookies))
	}
	if cookies[0].Domain != ".xiaohongshu.com" || cookies[0].Name != "web_session" || cookies[0].Value != "secret" {
		t.Fatalf("HttpOnly cookie parsed incorrectly: %+v", cookies[0])
	}
	client, err := httpx.New(httpx.Options{Cookies: cookies})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse("https://www.xiaohongshu.com/discovery/item/example")
	if got := client.HTTP().Jar.Cookies(u); len(got) != 2 {
		t.Fatalf("request sends %d cookies, want 2", len(got))
	}
}
