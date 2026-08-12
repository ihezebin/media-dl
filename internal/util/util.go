package util

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
)

var urlInText = regexp.MustCompile(`https?://[^\s<>"']+`)

// ExtractFirstURL 从分享文案中提取第一个 URL。
func ExtractFirstURL(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		return strings.Fields(text)[0]
	}
	m := urlInText.FindString(text)
	return strings.TrimRight(m, ".,);]")
}

// SanitizeFilename 去掉不安全文件名字符。
func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "untitled"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r < 32:
			continue
		case strings.ContainsRune(`\/:*?"<>|`, r):
			b.WriteByte('_')
		case unicode.IsSpace(r):
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), " ._")
	runes := []rune(out)
	if len(runes) > 80 {
		out = string(runes[:80])
	}
	if out == "" {
		return "untitled"
	}
	return out
}

// ExtractBalancedJSON 从 text[start:] 提取第一个完整 JSON 对象。
func ExtractBalancedJSON(text string, start int) (json.RawMessage, error) {
	if start < 0 || start >= len(text) {
		return nil, ErrNotFound
	}
	i := strings.IndexByte(text[start:], '{')
	if i < 0 {
		return nil, ErrNotFound
	}
	i += start
	depth := 0
	inStr := false
	esc := false
	for j := i; j < len(text) && j < i+2_000_000; j++ {
		c := text[j]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return json.RawMessage(text[i : j+1]), nil
			}
		}
	}
	return nil, ErrNotFound
}

// ErrNotFound 未找到目标数据。
var ErrNotFound = errString("not found")

type errString string

func (e errString) Error() string { return string(e) }
