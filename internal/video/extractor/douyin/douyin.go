package douyin

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/util"
	"github.com/hezebin/media-dl/internal/video/extractor"
	"github.com/hezebin/media-dl/internal/video/model"
)

func init() {
	extractor.Register(&Extractor{})
}

var (
	shareURLRe = regexp.MustCompile(`(?i)https?://(?:www\.)?(?:douyin\.com|iesdouyin\.com)/(?:video|note|share/video)/(\d+)`)
	shortURLRe = regexp.MustCompile(`(?i)https?://v\.douyin\.com/[\w-]+/?`)
	anyIDRe    = regexp.MustCompile(`/(\d{15,})`)
)

const (
	douyinShareUA    = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1"
	webSignatureSalt = "A96D855A08C0A9707F8BEF0D9A527E4E"
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
	var errs []string
	// 1) iesdouyin 分享页 SSR（旧路径；2026-08 起常不再内嵌 videoInfoRes）
	if item, err := fetchDetailSharePage(client, awemeID); err == nil {
		return item, nil
	} else {
		errs = append(errs, err.Error())
	}
	// 2) douyin.com 视频页 RENDER_DATA
	if item, err := fetchDetailWebPage(client, awemeID); err == nil {
		return item, nil
	} else {
		errs = append(errs, err.Error())
	}
	// 3) Web detail API + a_bogus（分享页去 SSR 后的主路径）
	if item, err := fetchDetailWebAPI(client, awemeID); err == nil {
		return item, nil
	} else {
		errs = append(errs, err.Error())
	}
	return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
}

func fetchDetailSharePage(client *httpx.Client, awemeID string) (map[string]any, error) {
	headers := map[string]string{
		"User-Agent": douyinShareUA,
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
	return nil, fmt.Errorf("分享页 SSR 无视频字段 (平台已改为客户端拉取): %s", awemeID)
}

func ensureTTWid(client *httpx.Client) error {
	u, _ := url.Parse("https://www.douyin.com/")
	for _, c := range client.HTTP().Jar.Cookies(u) {
		if c.Name == "ttwid" && c.Value != "" {
			return nil
		}
	}
	const payload = `{"region":"cn","aid":1768,"needFid":false,"service":"www.ixigua.com","migrate_info":{"ticket":"","source":"node"},"cbUrlProtocol":"https","union":true}`
	req, err := http.NewRequest(http.MethodPost, "https://ttwid.bytedance.com/ttwid/union/register/", bytes.NewBufferString(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", webAPIUA)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("注册 ttwid 失败: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	ttwid := ""
	for _, c := range resp.Cookies() {
		if c.Name == "ttwid" {
			ttwid = c.Value
			break
		}
	}
	if ttwid == "" {
		return fmt.Errorf("未拿到 ttwid")
	}
	// 写入 douyin / iesdouyin，供后续 API 使用
	for _, host := range []string{"www.douyin.com", "www.iesdouyin.com"} {
		hu, _ := url.Parse("https://" + host + "/")
		client.HTTP().Jar.SetCookies(hu, []*http.Cookie{{
			Name:   "ttwid",
			Value:  ttwid,
			Path:   "/",
			Domain: strings.TrimPrefix(host, "www."),
		}})
	}
	return nil
}

func fetchDetailWebAPI(client *httpx.Client, awemeID string) (map[string]any, error) {
	if err := ensureTTWid(client); err != nil {
		return nil, err
	}
	params := defaultWebParams(awemeID)
	params["request_source"] = "600"
	params["origin_type"] = "general"
	query := encodeWebParams(params)
	aBogus := generateABogus(query)
	query += "&a_bogus=" + webQueryEscape(aBogus)
	query, signatureHeaders := addWebSignature(client, query)
	apiURL := "https://www.douyin.com/aweme/v1/web/aweme/detail/?" + query

	headers := map[string]string{
		"User-Agent": webAPIUA,
		"Referer":    "https://www.douyin.com/",
		"Accept":     "application/json, text/plain, */*",
	}
	for k, v := range signatureHeaders {
		headers[k] = v
	}
	body, resp, err := client.GetString(apiURL, headers)
	if err != nil {
		return nil, fmt.Errorf("web detail 请求失败: %w", err)
	}
	if resp.StatusCode >= 400 {
		if strings.Contains(body, "ArgusSecurityPlugin") || strings.Contains(strings.ToLower(body), "uifid") {
			return nil, fmt.Errorf("web detail HTTP %d：抖音要求浏览器访客签名，请导入当前抖音 Cookie（需包含 uifid）后重试", resp.StatusCode)
		}
		return nil, fmt.Errorf("web detail HTTP %d", resp.StatusCode)
	}
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("web detail 返回空 (可尝试 --cookies 导入浏览器 Cookie): %s", awemeID)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil, fmt.Errorf("web detail JSON 解析失败: %w", err)
	}
	if detail, ok := data["aweme_detail"].(map[string]any); ok && detail != nil {
		return detail, nil
	}
	if code, ok := data["status_code"]; ok {
		return nil, fmt.Errorf("web detail status_code=%v msg=%v", code, data["status_msg"])
	}
	return nil, fmt.Errorf("web detail 无 aweme_detail: %s", awemeID)
}

// addWebSignature adds the visitor-bound signature required by the current
// Douyin web detail endpoint. A-Bogus alone is no longer sufficient: Douyin
// binds this signature to the uifid cookie minted by its own web page.
func addWebSignature(client *httpx.Client, query string) (string, map[string]string) {
	cookies := client.HTTP().Jar.Cookies(&url.URL{Scheme: "https", Host: "www.douyin.com", Path: "/"})
	uifid, verifyFP := "", ""
	for _, c := range cookies {
		switch strings.ToLower(c.Name) {
		case "uifid", "uifid_temp", "uifidtemp":
			if uifid == "" {
				uifid = c.Value
			}
		case "s_v_web_id":
			verifyFP = c.Value
		}
	}
	if uifid == "" {
		return query, nil
	}
	if verifyFP != "" {
		query += "&verifyFp=" + webQueryEscape(verifyFP) + "&fp=" + webQueryEscape(verifyFP)
	}
	query += "&uifid=" + webQueryEscape(uifid)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	query += "&timestamp=" + timestamp
	digest := md5.Sum([]byte(uifid + "_" + timestamp + "_" + webSignatureSalt + "_" + query))
	signature := fmt.Sprintf("%x", digest)
	query += "&x-secsdk-web-signature=" + signature
	return query, map[string]string{
		"uifid":                  uifid,
		"x-secsdk-web-signature": signature,
		"x-secsdk-web-expire":    timestamp,
	}
}

func webQueryEscape(value string) string {
	// URLSearchParams uses the application/x-www-form-urlencoded percent-encode
	// set: spaces are %20 and ~ is escaped, while *-._ remain unchanged.
	value = url.QueryEscape(value)
	value = strings.ReplaceAll(value, "+", "%20")
	return strings.ReplaceAll(value, "~", "%7E")
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
