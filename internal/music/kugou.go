package music

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hezebin/media-dl/internal/httpx"
)

var kugouHashInHTML = regexp.MustCompile(`(?i)["']hash["']\s*:\s*["']([a-f0-9]{32})["']`)
var kugouHashValue = regexp.MustCompile(`(?i)^[a-f0-9]{32}$`)

func resolveKugouURL(client *httpx.Client, rawURL string) (string, error) {
	if !isKugouExtendedURL(rawURL) {
		return rawURL, nil
	}
	if client == nil {
		var err error
		client, err = httpx.New(httpx.Options{})
		if err != nil {
			return "", err
		}
	}
	body, _, err := client.GetString(rawURL, map[string]string{
		"Referer":    "https://www.kugou.com/",
		"Accept":     "text/html,application/xhtml+xml",
		"User-Agent": httpx.DefaultUA,
	})
	if err != nil {
		return "", fmt.Errorf("读取酷狗分享页失败: %w", err)
	}
	hash, err := extractKugouHashFromHTML(body)
	if err != nil {
		return "", fmt.Errorf("酷狗分享页未找到歌曲 hash: %w", err)
	}
	return "https://www.kugou.com/song/#hash=" + hash, nil
}

func isKugouExtendedURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host != "kugou.com" && !strings.HasSuffix(host, ".kugou.com") {
		return false
	}
	path := strings.ToLower(u.Path)
	return strings.HasPrefix(path, "/mixsong/") ||
		path == "/share/song.html" ||
		strings.HasPrefix(path, "/share/")
}

func extractKugouHashFromHTML(body string) (string, error) {
	const marker = "dataFromSmarty"
	index := strings.Index(body, marker)
	if index >= 0 {
		if raw, ok := balancedArray(body[index:]); ok {
			var items []struct {
				Hash string `json:"hash"`
			}
			if err := json.Unmarshal([]byte(raw), &items); err == nil {
				for _, item := range items {
					if validKugouHash(item.Hash) {
						return strings.ToUpper(item.Hash), nil
					}
				}
			}
		}
	}
	if match := kugouHashInHTML.FindStringSubmatch(body); len(match) >= 2 {
		return strings.ToUpper(match[1]), nil
	}
	return "", fmt.Errorf("dataFromSmarty 中没有有效的 32 位 hash")
}

func balancedArray(text string) (string, bool) {
	start := strings.IndexByte(text, '[')
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '\\':
			if inString {
				escaped = !escaped
			}
		case '"':
			if !escaped {
				inString = !inString
			}
			escaped = false
		default:
			escaped = false
		}
		if inString {
			continue
		}
		switch text[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return text[start : i+1], true
			}
		}
	}
	return "", false
}

func validKugouHash(hash string) bool {
	return kugouHashValue.MatchString(strings.TrimSpace(hash))
}
