package util

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
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

// StripJSONP 去掉 JSONP 包装，提取其中 JSON 对象。
func StripJSONP(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '{'); i >= 0 {
		if raw, err := ExtractBalancedJSON(s, i); err == nil {
			return string(raw)
		}
		if j := strings.LastIndexByte(s, '}'); j > i {
			return s[i : j+1]
		}
	}
	return s
}

func AsMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func AsSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

// Nested 按路径取值，中途遇到非 object 则返回 nil。
func Nested(v any, keys ...string) any {
	cur := v
	for _, k := range keys {
		m := AsMap(cur)
		if m == nil {
			return nil
		}
		cur = m[k]
	}
	return cur
}

func AsString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		if t == nil {
			return ""
		}
		return fmt.Sprint(t)
	}
}

func ToInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case json.Number:
		i, _ := t.Int64()
		return i
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		i, _ := strconv.ParseInt(t, 10, 64)
		return i
	default:
		return 0
	}
}

func ToFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case json.Number:
		f, _ := t.Float64()
		return f
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}

func FirstString(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// ErrNotFound 未找到目标数据。
var ErrNotFound = errString("not found")

type errString string

func (e errString) Error() string { return string(e) }
