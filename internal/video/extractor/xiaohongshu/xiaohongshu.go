package xiaohongshu

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/util"
	"github.com/hezebin/media-dl/internal/video/extractor"
	"github.com/hezebin/media-dl/internal/video/model"
)

func init() {
	extractor.Register(&Extractor{})
}

// 参考 yt-dlp xiaohongshu extractor：抓取页面 window.__INITIAL_STATE__

var (
	noteIDRe   = regexp.MustCompile(`(?i)/(?:explore|discovery/item)/([0-9a-f]+)`)
	shortURLRe = regexp.MustCompile(`(?i)https?://(?:www\.)?xhslink\.com/[\w/-]+`)
	initState  = regexp.MustCompile(`window\.__INITIAL_STATE__\s*=`)
)

type Extractor struct{}

func (e *Extractor) Name() string { return "xiaohongshu" }

func (e *Extractor) Match(rawURL string) bool {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = rawURL
	}
	lu := strings.ToLower(u)
	return strings.Contains(lu, "xiaohongshu.com") || strings.Contains(lu, "xhslink.com")
}

func (e *Extractor) Extract(client *httpx.Client, rawURL string) (*model.VideoInfo, error) {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = strings.TrimSpace(rawURL)
	}
	pageURL, err := resolveURL(client, u)
	if err != nil {
		return nil, err
	}
	noteID := ""
	if m := noteIDRe.FindStringSubmatch(pageURL); len(m) > 1 {
		noteID = m[1]
	}
	if noteID == "" {
		return nil, fmt.Errorf("无法解析笔记 ID: %s", pageURL)
	}

	headers := map[string]string{
		"Referer":         "https://www.xiaohongshu.com/",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9",
	}
	body, resp, err := client.GetString(pageURL, headers)
	if err != nil {
		return nil, fmt.Errorf("请求笔记页失败: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("笔记页 HTTP %d", resp.StatusCode)
	}
	finalURL := resp.Request.URL.String()
	if strings.Contains(finalURL, "/404") || strings.Contains(body, "当前笔记暂时无法浏览") {
		return nil, fmt.Errorf("笔记不可访问或已失效，请使用带 xsec_token 的完整分享链接，或提供 --cookie")
	}

	idx := initState.FindStringIndex(body)
	if idx == nil {
		return nil, fmt.Errorf("未找到 __INITIAL_STATE__（可能需要登录 Cookie 或完整带 xsec_token 的链接）")
	}
	raw, err := util.ExtractBalancedJSON(body, idx[1])
	if err != nil {
		return nil, fmt.Errorf("解析 INITIAL_STATE 失败: %w", err)
	}
	// 小红书偶发使用 undefined，替换成 null 便于 JSON 解析
	cleaned := strings.ReplaceAll(string(raw), "undefined", "null")

	var state map[string]any
	if err := json.Unmarshal([]byte(cleaned), &state); err != nil {
		return nil, fmt.Errorf("INITIAL_STATE JSON 失败: %w", err)
	}

	note := findNote(state, noteID)
	if note == nil {
		return nil, fmt.Errorf("未找到笔记详情: %s", noteID)
	}

	info := &model.VideoInfo{
		Platform:   "xiaohongshu",
		ID:         noteID,
		WebpageURL: pageURL,
	}
	if t, ok := note["title"].(string); ok {
		info.Title = t
	}
	if d, ok := note["desc"].(string); ok {
		info.Description = d
		if info.Title == "" {
			info.Title = d
		}
	}
	if info.Title == "" {
		info.Title = noteID
	}
	if user, ok := note["user"].(map[string]any); ok {
		if n, ok := user["nickname"].(string); ok {
			info.Author = n
		}
		if id, ok := user["userId"].(string); ok {
			info.AuthorID = id
		}
	}

	// 封面 / 图片
	if images, ok := note["imageList"].([]any); ok {
		for _, img := range images {
			im, ok := img.(map[string]any)
			if !ok {
				continue
			}
			for _, key := range []string{"urlDefault", "urlPre"} {
				if u, ok := im[key].(string); ok && u != "" {
					if info.CoverURL == "" {
						info.CoverURL = u
					}
					break
				}
			}
		}
	}

	dlHeaders := map[string]string{
		"Referer": "https://www.xiaohongshu.com/",
	}

	// 视频流：video.media.stream.*.*
	if video, ok := note["video"].(map[string]any); ok {
		if media, ok := video["media"].(map[string]any); ok {
			if stream, ok := media["stream"].(map[string]any); ok {
				for _, group := range stream {
					list, ok := group.([]any)
					if !ok {
						continue
					}
					for _, item := range list {
						sm, ok := item.(map[string]any)
						if !ok {
							continue
						}
						info.Formats = append(info.Formats, streamFormats(sm, dlHeaders)...)
						if info.Duration == 0 {
							if d := toFloat(sm["duration"]); d > 0 {
								if d > 1000 {
									info.Duration = d / 1000
								} else {
									info.Duration = d
								}
							}
						}
					}
				}
			}
		}
		// 原片 originVideoKey
		if consumer, ok := video["consumer"].(map[string]any); ok {
			if key, ok := consumer["originVideoKey"].(string); ok && key != "" {
				originURL := "https://sns-video-bd.xhscdn.com/" + strings.TrimPrefix(key, "/")
				info.Formats = append(info.Formats, model.Format{
					FormatID:   "origin",
					URL:        originURL,
					Ext:        "mp4",
					HasVideo:   true,
					HasAudio:   true,
					Preference: 100000,
					Headers:    dlHeaders,
				})
			}
		}
	}

	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("该笔记没有可下载视频（可能是纯图文）: %s", noteID)
	}
	return info, nil
}

func resolveURL(client *httpx.Client, rawURL string) (string, error) {
	if shortURLRe.MatchString(rawURL) {
		final, err := client.ResolveRedirect(rawURL, map[string]string{
			"Referer": "https://www.xiaohongshu.com/",
		})
		if err != nil {
			return "", fmt.Errorf("解析小红书短链失败: %w", err)
		}
		return final, nil
	}
	// 保留 query（xsec_token 等）
	if _, err := url.Parse(rawURL); err != nil {
		return "", err
	}
	return rawURL, nil
}

func findNote(state map[string]any, noteID string) map[string]any {
	noteObj, _ := state["note"].(map[string]any)
	if noteObj == nil {
		return nil
	}
	if m, ok := noteObj["noteDetailMap"].(map[string]any); ok {
		if n, ok := m[noteID].(map[string]any); ok {
			if note, ok := n["note"].(map[string]any); ok {
				return note
			}
			return n
		}
		// 有时 key 不完全一致，取第一个
		for _, v := range m {
			if n, ok := v.(map[string]any); ok {
				if note, ok := n["note"].(map[string]any); ok {
					return note
				}
			}
		}
	}
	return nil
}

func streamFormats(sm map[string]any, headers map[string]string) []model.Format {
	var out []model.Format
	w := int(toInt64(sm["width"]))
	h := int(toInt64(sm["height"]))
	quality, _ := sm["qualityType"].(string)
	ext := "mp4"
	add := func(u string, pref int) {
		if u == "" {
			return
		}
		out = append(out, model.Format{
			FormatID:   quality,
			URL:        u,
			Ext:        ext,
			Width:      w,
			Height:     h,
			Filesize:   toInt64(sm["size"]),
			VCodec:     fmt.Sprintf("%v", sm["videoCodec"]),
			ACodec:     fmt.Sprintf("%v", sm["audioCodec"]),
			HasVideo:   true,
			HasAudio:   true,
			Quality:    quality,
			Preference: pref + h,
			Headers:    headers,
		})
	}
	if u, ok := sm["masterUrl"].(string); ok {
		add(u, 1000)
	}
	if backups, ok := sm["backupUrls"].([]any); ok {
		for i, b := range backups {
			if s, ok := b.(string); ok {
				add(s, 100-i)
			}
		}
	}
	return out
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		var n int64
		fmt.Sscan(t, &n)
		return n
	default:
		return 0
	}
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		var f float64
		fmt.Sscan(t, &f)
		return f
	default:
		return 0
	}
}
