package douyin

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hezebin/media-dl/internal/extractor"
	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/model"
	"github.com/hezebin/media-dl/internal/util"
)

func init() {
	extractor.Register(&Extractor{})
}

var (
	shareURLRe = regexp.MustCompile(`(?i)https?://(?:www\.)?(?:douyin\.com|iesdouyin\.com)/(?:video|note|share/video)/(\d+)`)
	shortURLRe = regexp.MustCompile(`(?i)https?://v\.douyin\.com/[\w-]+/?`)
	anyIDRe    = regexp.MustCompile(`/(\d{15,})`)
)

type Extractor struct{}

func (e *Extractor) Name() string { return "douyin" }

func (e *Extractor) Match(rawURL string) bool {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = rawURL
	}
	lu := strings.ToLower(u)
	return strings.Contains(lu, "douyin.com") || strings.Contains(lu, "iesdouyin.com")
}

func (e *Extractor) Extract(client *httpx.Client, rawURL string) (*model.VideoInfo, error) {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = strings.TrimSpace(rawURL)
	}
	awemeID, err := resolveAwemeID(client, u)
	if err != nil {
		return nil, err
	}
	detail, err := fetchDetail(client, awemeID)
	if err != nil {
		return nil, err
	}
	return parseDetail(awemeID, detail)
}

func resolveAwemeID(client *httpx.Client, rawURL string) (string, error) {
	if m := shareURLRe.FindStringSubmatch(rawURL); len(m) > 1 {
		return m[1], nil
	}
	if shortURLRe.MatchString(rawURL) {
		finalURL, err := client.ResolveRedirect(rawURL, map[string]string{
			"Referer": "https://www.douyin.com/",
		})
		if err != nil {
			return "", fmt.Errorf("解析短链失败: %w", err)
		}
		if m := shareURLRe.FindStringSubmatch(finalURL); len(m) > 1 {
			return m[1], nil
		}
		if m := anyIDRe.FindStringSubmatch(finalURL); len(m) > 1 {
			return m[1], nil
		}
		// 跟随完整重定向后再看最终 URL（有些短链 Location 是中间页）
		resp, err := client.Get(rawURL, map[string]string{
			"Referer": "https://www.douyin.com/",
		})
		if err == nil {
			final := resp.Request.URL.String()
			resp.Body.Close()
			if m := shareURLRe.FindStringSubmatch(final); len(m) > 1 {
				return m[1], nil
			}
			if m := anyIDRe.FindStringSubmatch(final); len(m) > 1 {
				return m[1], nil
			}
		}
		return "", fmt.Errorf("无法从短链解析视频 ID: %s -> %s", rawURL, finalURL)
	}
	if m := anyIDRe.FindStringSubmatch(rawURL); len(m) > 1 {
		return m[1], nil
	}
	if regexp.MustCompile(`^\d{15,}$`).MatchString(rawURL) {
		return rawURL, nil
	}
	return "", fmt.Errorf("无法识别抖音链接: %s", rawURL)
}

func fetchDetail(client *httpx.Client, awemeID string) (map[string]any, error) {
	// 1) iesdouyin 分享页 SSR（无需 X-Bogus）
	if item, err := fetchDetailSharePage(client, awemeID); err == nil {
		return item, nil
	} else {
		shareErr := err
		// 2) 尝试 douyin.com 视频页 RENDER_DATA
		if item, err := fetchDetailWebPage(client, awemeID); err == nil {
			return item, nil
		}
		return nil, shareErr
	}
}

func fetchDetailSharePage(client *httpx.Client, awemeID string) (map[string]any, error) {
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
		"Referer":    "https://www.douyin.com/",
		"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	}
	pageURL := fmt.Sprintf("https://www.iesdouyin.com/share/video/%s/", awemeID)
	body, resp, err := client.GetString(pageURL, headers)
	if err != nil {
		return nil, fmt.Errorf("请求分享页失败: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("分享页 HTTP %d", resp.StatusCode)
	}

	idx := strings.Index(body, "_ROUTER_DATA")
	if idx < 0 {
		idx = strings.Index(body, "RENDER_DATA")
	}
	if idx < 0 {
		return nil, fmt.Errorf("分享页未找到 SSR 数据 (可能需要登录 Cookie 或国内网络): %s", awemeID)
	}
	raw, err := util.ExtractBalancedJSON(body, idx)
	if err != nil {
		return nil, fmt.Errorf("解析 SSR JSON 失败: %w", err)
	}
	decoded := string(raw)
	if strings.Contains(decoded, "%7B") || strings.HasPrefix(strings.TrimSpace(decoded), "%") {
		if unescaped, e := url.QueryUnescape(decoded); e == nil {
			decoded = unescaped
		}
	}
	decoded = strings.ReplaceAll(decoded, `\u002F`, "/")

	var data map[string]any
	if err := json.Unmarshal([]byte(decoded), &data); err != nil {
		return nil, fmt.Errorf("SSR JSON 反序列化失败: %w", err)
	}
	if item := findAwemeItem(data); item != nil {
		return item, nil
	}
	return nil, fmt.Errorf("视频数据为空 (海外 IP 可能被拦截，可尝试 --proxy): %s", awemeID)
}

func fetchDetailWebPage(client *httpx.Client, awemeID string) (map[string]any, error) {
	headers := map[string]string{
		"User-Agent": httpx.DefaultUA,
		"Referer":    "https://www.douyin.com/",
		"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	}
	pageURL := fmt.Sprintf("https://www.douyin.com/video/%s", awemeID)
	body, resp, err := client.GetString(pageURL, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("视频页 HTTP %d", resp.StatusCode)
	}
	// <script id="RENDER_DATA" type="application/json">url-encoded</script>
	re := regexp.MustCompile(`(?s)<script id="RENDER_DATA"[^>]*>([^<]+)</script>`)
	m := re.FindStringSubmatch(body)
	if len(m) < 2 {
		idx := strings.Index(body, "RENDER_DATA")
		if idx < 0 {
			return nil, fmt.Errorf("无 RENDER_DATA")
		}
		raw, err := util.ExtractBalancedJSON(body, idx)
		if err != nil {
			return nil, err
		}
		m = []string{"", string(raw)}
	}
	decoded, err := url.QueryUnescape(strings.TrimSpace(m[1]))
	if err != nil {
		decoded = m[1]
	}
	decoded = strings.ReplaceAll(decoded, `\u002F`, "/")
	var data map[string]any
	if err := json.Unmarshal([]byte(decoded), &data); err != nil {
		return nil, err
	}
	if item := findAwemeItem(data); item != nil {
		return item, nil
	}
	// 深度搜索 aweme_detail / awemeId
	if item := deepFindAweme(data, awemeID); item != nil {
		return item, nil
	}
	return nil, fmt.Errorf("RENDER_DATA 中无视频")
}

func deepFindAweme(v any, awemeID string) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		if id, ok := t["aweme_id"].(string); ok && id == awemeID {
			return t
		}
		if id, ok := t["awemeId"].(string); ok && id == awemeID {
			return t
		}
		for _, child := range t {
			if found := deepFindAweme(child, awemeID); found != nil {
				return found
			}
		}
	case []any:
		for _, child := range t {
			if found := deepFindAweme(child, awemeID); found != nil {
				return found
			}
		}
	}
	return nil
}

func findAwemeItem(data map[string]any) map[string]any {
	if loader, ok := data["loaderData"].(map[string]any); ok {
		for _, v := range loader {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if videoRes, ok := m["videoInfoRes"].(map[string]any); ok {
				if items, ok := videoRes["item_list"].([]any); ok && len(items) > 0 {
					if item, ok := items[0].(map[string]any); ok {
						return item
					}
				}
			}
		}
	}
	// 兼容其它嵌套
	if items, ok := data["item_list"].([]any); ok && len(items) > 0 {
		if item, ok := items[0].(map[string]any); ok {
			return item
		}
	}
	if detail, ok := data["aweme_detail"].(map[string]any); ok {
		return detail
	}
	return nil
}

func parseDetail(awemeID string, detail map[string]any) (*model.VideoInfo, error) {
	info := &model.VideoInfo{
		Platform:   "douyin",
		ID:         awemeID,
		WebpageURL: fmt.Sprintf("https://www.douyin.com/video/%s", awemeID),
	}
	if desc, ok := detail["desc"].(string); ok {
		info.Title = desc
		info.Description = desc
	}
	if author, ok := detail["author"].(map[string]any); ok {
		if n, ok := author["nickname"].(string); ok {
			info.Author = n
		}
		if id, ok := author["uid"].(string); ok {
			info.AuthorID = id
		} else if id, ok := author["unique_id"].(string); ok {
			info.AuthorID = id
		}
	}
	if info.Title == "" {
		info.Title = awemeID
	}

	video, _ := detail["video"].(map[string]any)
	info.CoverURL = firstURL(nestedMap(video, "origin_cover"))
	if info.CoverURL == "" {
		info.CoverURL = firstURL(nestedMap(video, "cover"))
	}
	if info.CoverURL == "" {
		info.CoverURL = firstURL(nestedMap(video, "dynamic_cover"))
	}
	// duration 常见为毫秒
	if d := toFloat(detail["duration"]); d > 0 {
		if d > 1000 {
			info.Duration = d / 1000
		} else {
			info.Duration = d
		}
	} else if d := toFloat(video["duration"]); d > 0 {
		if d > 1000 {
			info.Duration = d / 1000
		} else {
			info.Duration = d
		}
	}

	playAddr := nestedMap(video, "play_addr")
	urls := urlList(playAddr)
	if len(urls) == 0 {
		// 部分结构用 play_addr_h264 / download_addr
		urls = urlList(nestedMap(video, "play_addr_h264"))
	}
	if len(urls) == 0 {
		urls = urlList(nestedMap(video, "download_addr"))
	}
	if len(urls) > 0 {
		// 取最后一个通常质量更高；playwm → play 去水印
		vURL := strings.ReplaceAll(urls[len(urls)-1], "playwm", "play")
		info.Formats = append(info.Formats, model.Format{
			FormatID:   "no_watermark",
			URL:        vURL,
			Ext:        "mp4",
			HasVideo:   true,
			HasAudio:   true,
			Preference: 100,
			Headers: map[string]string{
				"Referer": "https://www.douyin.com/",
			},
		})
	}

	// 图集
	if images, ok := detail["images"].([]any); ok && len(images) > 0 {
		for i, img := range images {
			im, ok := img.(map[string]any)
			if !ok {
				continue
			}
			ul := urlList(im)
			if len(ul) == 0 {
				continue
			}
			info.Formats = append(info.Formats, model.Format{
				FormatID:   fmt.Sprintf("image_%d", i+1),
				URL:        ul[len(ul)-1],
				Ext:        "jpg",
				HasVideo:   false,
				HasAudio:   false,
				Preference: 10,
				Headers: map[string]string{
					"Referer": "https://www.douyin.com/",
				},
			})
		}
	}

	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("未找到可下载媒体: %s", awemeID)
	}
	return info, nil
}

func nestedMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, _ := m[key].(map[string]any)
	return v
}

func urlList(m map[string]any) []string {
	if m == nil {
		return nil
	}
	raw, ok := m["url_list"].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, u := range raw {
		if s, ok := u.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func firstURL(m map[string]any) string {
	ul := urlList(m)
	if len(ul) == 0 {
		return ""
	}
	return ul[0]
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
