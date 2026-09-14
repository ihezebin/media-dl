package music

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/hezebin/media-dl/internal/httpx"
)

const appleMusicHomepage = "https://music.apple.com"

var (
	appleIndexScript = regexp.MustCompile(`(?i)(/assets/index(?:-legacy)?[~\-][^/"']+\.js)`)
	appleTokenValue  = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
)

func (s *Service) appleCookieValue() (string, error) {
	if cookieValue(s.cookie, "token") != "" {
		return s.cookie, nil
	}

	s.appleTokenOnce.Do(func() {
		client, err := httpx.New(httpx.Options{
			Timeout: 30 * time.Second,
			Cookies: httpx.ParseCookieHeader(s.cookie, "music.apple.com"),
		})
		if err != nil {
			s.appleTokenErr = err
			return
		}
		body, resp, err := client.GetString(appleMusicHomepage, appleHeaders())
		if err != nil {
			s.appleTokenErr = fmt.Errorf("Apple Music 首页请求失败: %w", err)
			return
		}
		if err := checkAppleResponse(resp); err != nil {
			s.appleTokenErr = fmt.Errorf("Apple Music 首页请求失败: %w", err)
			return
		}
		match := appleIndexScript.FindStringSubmatch(body)
		if len(match) < 2 {
			s.appleTokenErr = fmt.Errorf("Apple Music 首页未找到前端脚本")
			return
		}

		scriptBody, scriptResp, err := client.GetString(appleMusicHomepage+match[1], appleHeaders())
		if err != nil {
			s.appleTokenErr = fmt.Errorf("Apple Music 前端脚本请求失败: %w", err)
			return
		}
		if err := checkAppleResponse(scriptResp); err != nil {
			s.appleTokenErr = fmt.Errorf("Apple Music 前端脚本请求失败: %w", err)
			return
		}
		token := appleTokenValue.FindString(scriptBody)
		if token == "" {
			s.appleTokenErr = fmt.Errorf("Apple Music 前端脚本未找到 bearer token")
			return
		}
		s.appleCookie = appendCookie(s.cookie, "token", token)
	})
	if s.appleTokenErr != nil {
		return "", s.appleTokenErr
	}
	return s.appleCookie, nil
}

func cookieValue(raw, name string) string {
	for _, cookie := range httpx.ParseCookieHeader(raw, "") {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func appendCookie(raw, name, value string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return name + "=" + value
	}
	return raw + "; " + name + "=" + value
}

func appleHeaders() map[string]string {
	return map[string]string{
		"Accept":     "text/html,application/xhtml+xml",
		"User-Agent": httpx.DefaultUA,
	}
}

func checkAppleResponse(resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("空响应")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
