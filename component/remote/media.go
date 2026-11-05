package remote

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hezebin/media-dl/config"
	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/music"
	"github.com/hezebin/media-dl/internal/video/downloader"
	"github.com/hezebin/media-dl/internal/video/extractor"
)

var SharedCookieDomains = []string{
	"douyin.com", "bilibili.com", "xiaohongshu.com", "weibo.com", "weibo.cn", "youku.com", "tudou.com",
	"iqiyi.com", "iq.com", "ixigua.com", "toutiao.com", "qq.com", "kugou.com", "5sing.kugou.com", "kuwo.cn",
	"migu.cn", "music.163.com", "qqmusic.qq.com", "qishui.com", "jamendo.com", "joox.com", "apple.com",
	"music.apple.com", "youtube.com", "youtu.be", "tiktok.com", "kuaishou.com", "kuaishouapp.com", "kwai.com",
	"baidu.com", "x.com", "twitter.com", "douyu.com", "huya.com",
}

func ApplyCookieHeader(client *httpx.Client, raw string) {
	if client == nil || strings.TrimSpace(raw) == "" {
		return
	}
	for _, domain := range SharedCookieDomains {
		u, err := url.Parse("https://" + domain + "/")
		if err != nil {
			continue
		}
		client.HTTP().Jar.SetCookies(u, httpx.ParseCookieHeader(raw, domain))
	}
}

func ApplyCookieFile(client *httpx.Client, raw string) {
	if client == nil || strings.TrimSpace(raw) == "" {
		return
	}
	for _, domain := range SharedCookieDomains {
		u, err := url.Parse("https://" + domain + "/")
		if err != nil {
			continue
		}
		client.HTTP().Jar.SetCookies(u, downloader.LoadCookiesText(raw))
	}
}

func CookieHeaderFromText(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	return CookieHeaderFromCookies(downloader.LoadCookiesText(raw))
}

func CookieHeaderFromTextForPlatform(raw, platform string) (string, error) {
	cookies := downloader.LoadCookiesText(raw)
	canon, err := music.NormalizePlatform(platform)
	if err != nil {
		return "", err
	}
	allowed := musicCookieDomains[canon]
	filtered := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" {
			continue
		}
		if cookie.Domain == "" || CookieDomainMatches(cookie.Domain, allowed) {
			filtered = append(filtered, cookie)
		}
	}
	if len(filtered) == 0 {
		return "", nil
	}
	return CookieHeaderFromCookies(filtered)
}

func CookieHeaderFromCookies(cookies []*http.Cookie) (string, error) {
	if len(cookies) == 0 {
		return "", fmt.Errorf("Cookie 文件中未找到有效 Cookie")
	}
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie != nil && cookie.Name != "" {
			parts = append(parts, cookie.Name+"="+cookie.Value)
		}
	}
	return strings.Join(parts, "; "), nil
}

func CookieDomainMatches(domain string, allowed []string) bool {
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	for _, suffix := range allowed {
		suffix = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(suffix)), ".")
		if domain == suffix || strings.HasSuffix(domain, "."+suffix) {
			return true
		}
	}
	return false
}

func RequestMusicCookie(raw, file string) (string, error) {
	if strings.TrimSpace(file) != "" {
		return CookieHeaderFromText(file)
	}
	return strings.TrimSpace(raw), nil
}

func DetectVideoPlatform(rawURL string) string {
	for _, item := range extractor.All() {
		if item.Match(rawURL) {
			return item.Name()
		}
	}
	return ""
}

func NewMediaClient(ctx context.Context, cfg config.Config) (*httpx.Client, error) {
	var cookies []*http.Cookie
	if cfg.CookiesFile != "" {
		loaded, err := downloader.LoadCookiesFile(cfg.CookiesFile)
		if err != nil {
			return nil, fmt.Errorf("读取 cookies 文件失败: %w", err)
		}
		cookies = append(cookies, loaded...)
	}
	if raw := strings.TrimSpace(cfg.Cookie); raw != "" {
		for _, domain := range SharedCookieDomains {
			cookies = append(cookies, httpx.ParseCookieHeader(raw, domain)...)
		}
	}
	return httpx.New(httpx.Options{Proxy: cfg.Proxy, Cookies: cookies, Context: ctx})
}

var musicCookieDomains = map[string][]string{
	"netease":  {"163.com"},
	"qq":       {"qq.com"},
	"kugou":    {"kugou.com"},
	"kuwo":     {"kuwo.cn", "kuwo.com"},
	"migu":     {"migu.cn"},
	"fivesing": {"5sing.com", "kugou.com"},
	"qianqian": {"baidu.com"},
	"soda":     {"qishui.com"},
	"jamendo":  {"jamendo.com"},
	"joox":     {"joox.com"},
	"bilibili": {"bilibili.com"},
	"apple":    {"apple.com"},
}
