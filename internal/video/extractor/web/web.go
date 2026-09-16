// Package web contains extractors for platforms whose playable media is
// exposed by the public page bootstrap data or a small public metadata API.
package web

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/util"
	"github.com/hezebin/media-dl/internal/video/extractor"
	"github.com/hezebin/media-dl/internal/video/model"
)

type platform struct {
	name   string
	match  func(string) bool
	id     func(string) string
	host   string
	pageUA string
}

type mediaCandidate struct {
	url     string
	key     string
	mime    string
	width   int
	height  int
	bitrate int
}

var platforms = []platform{
	{name: "youtube", match: matchYouTube, id: youtubeID, host: "https://www.youtube.com/"},
	{name: "tiktok", match: matchTikTok, id: numericID, host: "https://www.tiktok.com/"},
	{name: "kuaishou", match: matchKuaishou, id: kuaishouID, host: "https://www.kuaishou.com/"},
	{name: "baidu", match: matchBaidu, id: baiduID, host: "https://haokan.baidu.com/"},
	{name: "twitter", match: matchTwitter, id: twitterID, host: "https://x.com/", pageUA: httpx.DefaultUA},
	{name: "douyu", match: matchDouyu, id: douyuID, host: "https://www.douyu.com/"},
	{name: "huya", match: matchHuya, id: huyaID, host: "https://www.huya.com/"},
}

func init() {
	for _, item := range platforms {
		extractor.Register(&Extractor{config: item})
	}
}

type Extractor struct{ config platform }

func (e *Extractor) Name() string { return e.config.name }

func (e *Extractor) Match(rawURL string) bool {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = rawURL
	}
	return e.config.match(strings.ToLower(strings.TrimSpace(u)))
}

func (e *Extractor) Extract(client *httpx.Client, rawURL string) (*model.VideoInfo, error) {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = strings.TrimSpace(rawURL)
	}
	if u == "" {
		return nil, fmt.Errorf("视频链接为空")
	}

	pageURL, err := resolvePageURL(client, u, e.config)
	if err != nil {
		return nil, err
	}
	id := e.config.id(pageURL)
	if id == "" {
		id = e.config.id(u)
	}

	if e.config.name == "youtube" {
		return e.extractYouTube(client, pageURL, id)
	}
	if e.config.name == "twitter" {
		if info, twitterErr := e.extractTwitter(client, pageURL, id); twitterErr == nil {
			return info, nil
		}
	}

	headers := pageHeaders(e.config, pageURL)
	body, resp, err := client.GetString(pageURL, headers)
	if err != nil {
		return nil, fmt.Errorf("请求 %s 页面失败: %w", e.config.name, err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("%s 页面 HTTP %d", e.config.name, resp.StatusCode)
	}
	if resp.Request != nil && resp.Request.URL != nil {
		pageURL = resp.Request.URL.String()
		if id == "" {
			id = e.config.id(pageURL)
		}
	}

	info := pageInfo(e.config.name, id, pageURL, body)
	info.Formats = collectPageFormats(body, headers)
	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("未找到 %s 播放地址（可能需要登录 Cookie、地区网络或页面风控）", e.config.name)
	}
	if info.Title == "" {
		info.Title = info.ID
	}
	return info, nil
}

func resolvePageURL(client *httpx.Client, rawURL string, p platform) (string, error) {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		if isShortURL(rawURL, p.name) {
			resolved, err := client.ResolveRedirect(rawURL, map[string]string{"Referer": p.host})
			if err != nil {
				return "", fmt.Errorf("解析 %s 短链失败: %w", p.name, err)
			}
			return resolved, nil
		}
		return rawURL, nil
	}
	return "", fmt.Errorf("无法识别 %s 链接: %s", p.name, rawURL)
}

func isShortURL(rawURL, name string) bool {
	host := ""
	if parsed, err := url.Parse(rawURL); err == nil {
		host = strings.ToLower(parsed.Hostname())
	}
	switch name {
	case "youtube":
		return host == "youtu.be" || host == "www.youtube.com" && strings.HasPrefix(strings.ToLower(parsedPath(rawURL)), "/redirect")
	case "tiktok":
		return host == "vm.tiktok.com" || host == "vt.tiktok.com"
	case "kuaishou":
		return host == "v.kuaishou.com" || host == "v.kuaishouapp.com"
	case "twitter":
		return host == "t.co"
	default:
		return false
	}
}

func parsedPath(rawURL string) string {
	u, _ := url.Parse(rawURL)
	if u == nil {
		return ""
	}
	return u.Path
}

func pageHeaders(p platform, pageURL string) map[string]string {
	ua := p.pageUA
	if ua == "" {
		ua = httpx.DefaultUA
	}
	return map[string]string{
		"User-Agent":      ua,
		"Referer":         pageURL,
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
	}
}

func pageInfo(platformName, id, pageURL, body string) *model.VideoInfo {
	info := &model.VideoInfo{
		Platform:   platformName,
		ID:         id,
		WebpageURL: pageURL,
	}
	info.Title = firstNonEmpty(metaValue(body, "og:title"), metaValue(body, "twitter:title"), htmlTitle(body))
	info.Description = firstNonEmpty(metaValue(body, "og:description"), metaValue(body, "description"), metaValue(body, "twitter:description"))
	info.CoverURL = firstNonEmpty(metaValue(body, "og:image"), metaValue(body, "twitter:image"))
	info.Duration = parseDuration(metaValue(body, "video:duration"))
	if info.Title == "" {
		info.Title = id
	}
	return info
}

func collectPageFormats(body string, headers map[string]string) []model.Format {
	var candidates []mediaCandidate
	for _, value := range []struct{ key, value string }{
		{"og:video", metaValue(body, "og:video")},
		{"og:video:url", metaValue(body, "og:video:url")},
		{"twitter:player:stream", metaValue(body, "twitter:player:stream")},
	} {
		if u := mediaURL(value.value); u != "" {
			candidates = append(candidates, mediaCandidate{url: u, key: value.key})
		}
	}
	keyRe := regexp.MustCompile(`(?is)(?:playAddr|play_addr|downloadAddr|download_addr|videoUrl|video_url|playUrl|play_url|m3u8Url|m3u8_url|mp4Url|mp4_url|streamUrl|stream_url|sHlsUrl|sFlvUrl|hlsUrl|hls_url|flvUrl|flv_url|rtmpUrl|rtmp_url|videoSrc|contentUrl)\s*["']?\s*:\s*["']([^"']+)["']`)
	for _, match := range keyRe.FindAllStringSubmatch(body, -1) {
		if len(match) > 1 {
			if u := mediaURL(match[1]); u != "" {
				candidates = append(candidates, mediaCandidate{url: u, key: "page"})
			}
		}
	}
	for _, block := range jsonLDBlocks(body) {
		var value any
		if json.Unmarshal([]byte(block), &value) == nil {
			collectJSONCandidates(value, &candidates)
		}
	}

	seen := map[string]bool{}
	formats := make([]model.Format, 0, len(candidates))
	for _, item := range candidates {
		if item.url == "" || seen[item.url] || !looksLikeMediaURL(item.url, item.key) {
			continue
		}
		seen[item.url] = true
		ext, protocol := mediaExt(item.url)
		hasVideo := !strings.Contains(strings.ToLower(item.key), "audio")
		hasAudio := hasVideo && !strings.Contains(strings.ToLower(item.url), ".m3u8")
		formats = append(formats, model.Format{
			FormatID: "page_" + strconv.Itoa(len(formats)+1), URL: item.url, Ext: ext,
			Quality: qualityLabel(item.width, item.height), Width: item.width, Height: item.height,
			VCodec: videoCodec(item.mime, hasVideo), ACodec: audioCodec(item.mime, hasAudio),
			HasVideo: hasVideo, HasAudio: hasAudio, Headers: headers, Preference: item.bitrate,
			Protocol: protocol,
		})
	}
	sort.SliceStable(formats, func(i, j int) bool { return formatRank(formats[i]) > formatRank(formats[j]) })
	return formats
}

func collectJSONCandidates(value any, candidates *[]mediaCandidate) {
	// This function is intentionally kept as a small JSON-LD fallback. The
	// platform bootstrap formats are also handled by the raw-key scan above.
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			if s, ok := child.(string); ok && mediaKey(key) {
				if u := mediaURL(s); u != "" {
					*candidates = append(*candidates, mediaCandidate{url: u, key: key})
				}
			}
			collectJSONCandidates(child, candidates)
		}
	case []any:
		for _, child := range item {
			collectJSONCandidates(child, candidates)
		}
	}
}

func mediaKey(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "video") || strings.Contains(key, "playaddr") || strings.Contains(key, "playurl") || strings.Contains(key, "downloadaddr") || strings.Contains(key, "contenturl") || strings.Contains(key, "m3u8") || strings.Contains(key, "streamurl") || strings.Contains(key, "hls") || strings.Contains(key, "flv") || strings.Contains(key, "rtmp")
}

func jsonLDBlocks(body string) []string {
	re := regexp.MustCompile(`(?is)<script[^>]+type=["']application/ld\+json["'][^>]*>(.*?)</script>`)
	blocks := re.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if len(block) > 1 {
			out = append(out, strings.TrimSpace(html.UnescapeString(block[1])))
		}
	}
	return out
}

func metaValue(body, wanted string) string {
	tagRe := regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	attrRe := regexp.MustCompile(`(?is)([\w:-]+)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	wanted = strings.ToLower(wanted)
	for _, tag := range tagRe.FindAllString(body, -1) {
		attrs := map[string]string{}
		for _, match := range attrRe.FindAllStringSubmatch(tag, -1) {
			if len(match) > 3 {
				attrs[strings.ToLower(match[1])] = html.UnescapeString(firstNonEmpty(match[2], match[3]))
			}
		}
		property := strings.ToLower(firstNonEmpty(attrs["property"], attrs["name"]))
		if property == wanted {
			return strings.TrimSpace(attrs["content"])
		}
	}
	return ""
}

func htmlTitle(body string) string {
	match := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`).FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(strings.Join(strings.Fields(match[1]), " ")))
}

func mediaURL(raw string) string {
	raw = strings.TrimSpace(html.UnescapeString(raw))
	if raw == "" {
		return ""
	}
	if unquoted, err := strconv.Unquote(`"` + raw + `"`); err == nil {
		raw = unquoted
	}
	raw = strings.ReplaceAll(raw, `\u002F`, "/")
	raw = strings.ReplaceAll(raw, `\/`, "/")
	raw = strings.ReplaceAll(raw, `\u003F`, "?")
	raw = strings.ReplaceAll(raw, `\u003D`, "=")
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return ""
	}
	return raw
}

func looksLikeMediaURL(raw, key string) bool {
	low := strings.ToLower(raw)
	key = strings.ToLower(key)
	if strings.Contains(key, "video") || strings.Contains(key, "play") || strings.Contains(key, "stream") || strings.Contains(key, "content") || strings.Contains(key, "hls") || strings.Contains(key, "flv") || strings.Contains(key, "rtmp") {
		return true
	}
	return strings.Contains(low, ".mp4") || strings.Contains(low, ".m3u8") || strings.Contains(low, ".webm") || strings.Contains(low, ".m4s") || strings.Contains(low, "mime=video")
}

func mediaExt(raw string) (string, string) {
	low := strings.ToLower(raw)
	if strings.Contains(low, ".m3u8") || strings.Contains(low, "format=m3u8") {
		return "mp4", "m3u8"
	}
	if strings.Contains(low, ".webm") {
		return "webm", ""
	}
	if strings.Contains(low, ".flv") {
		return "flv", ""
	}
	return "mp4", ""
}

func qualityLabel(width, height int) string {
	if height > 0 {
		return strconv.Itoa(height) + "p"
	}
	if width > 0 {
		return strconv.Itoa(width) + "w"
	}
	return ""
}

func videoCodec(mime string, hasVideo bool) string {
	if !hasVideo {
		return "none"
	}
	if i := strings.Index(mime, "codecs="); i >= 0 {
		return strings.Trim(mime[i+7:], `"' `)
	}
	return ""
}

func audioCodec(mime string, hasAudio bool) string {
	if !hasAudio {
		return "none"
	}
	return ""
}

func formatRank(f model.Format) int { return f.Preference + f.Width*f.Height/1000 }

func parseDuration(raw string) float64 {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err == nil {
		return seconds
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (e *Extractor) extractYouTube(client *httpx.Client, pageURL, id string) (*model.VideoInfo, error) {
	headers := pageHeaders(e.config, pageURL)
	body, resp, err := client.GetString(pageURL, headers)
	if err != nil {
		return nil, fmt.Errorf("请求 YouTube 页面失败: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("YouTube 页面 HTTP %d", resp.StatusCode)
	}
	key := findCapture(body, `"INNERTUBE_API_KEY"\s*:\s*"([^"]+)"`)
	version := firstNonEmpty(findCapture(body, `"INNERTUBE_CLIENT_VERSION"\s*:\s*"([^"]+)"`), "2.20240814.01.00")
	if key == "" {
		key = "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8"
	}
	requestBody, _ := json.Marshal(map[string]any{
		"context": map[string]any{"client": map[string]string{"clientName": "WEB", "clientVersion": version, "hl": "zh-CN", "gl": "US"}},
		"videoId": id,
	})
	api := "https://www.youtube.com/youtubei/v1/player?key=" + url.QueryEscape(key)
	apiBody, apiResp, err := client.PostBytes(api, "application/json", requestBody, map[string]string{
		"Origin": "https://www.youtube.com", "Referer": pageURL, "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
	})
	if err != nil {
		return nil, fmt.Errorf("请求 YouTube 播放信息失败: %w", err)
	}
	if apiResp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("YouTube 播放信息 HTTP %d", apiResp.StatusCode)
	}
	var data map[string]any
	if err := json.Unmarshal(apiBody, &data); err != nil {
		return nil, fmt.Errorf("解析 YouTube 播放信息失败: %w", err)
	}
	info := &model.VideoInfo{Platform: "youtube", ID: id, WebpageURL: pageURL}
	if details := util.AsMap(data["videoDetails"]); details != nil {
		info.Title = util.FirstString(details, "title")
		info.Description = util.FirstString(details, "shortDescription", "description")
		info.Author = util.FirstString(details, "author")
		info.AuthorID = util.FirstString(details, "channelId")
		info.Duration = parseDuration(util.FirstString(details, "lengthSeconds"))
		if thumbs := util.AsMap(details["thumbnail"]); thumbs != nil {
			if list := util.AsSlice(thumbs["thumbnails"]); len(list) > 0 {
				info.CoverURL = util.FirstString(util.AsMap(list[len(list)-1]), "url")
			}
		}
	}
	info.Formats = youtubeFormats(data, headers)
	if len(info.Formats) == 0 {
		info.Formats = collectPageFormats(body, headers)
	}
	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("未找到 YouTube 可下载格式（可能需要 Cookie、PO Token 或地区网络）")
	}
	if info.Title == "" {
		info.Title = firstNonEmpty(metaValue(body, "og:title"), id)
	}
	if info.CoverURL == "" {
		info.CoverURL = "https://i.ytimg.com/vi/" + url.PathEscape(id) + "/hqdefault.jpg"
	}
	return info, nil
}

func youtubeFormats(data map[string]any, headers map[string]string) []model.Format {
	streaming := util.AsMap(data["streamingData"])
	if streaming == nil {
		return nil
	}
	var formats []model.Format
	for _, key := range []string{"formats", "adaptiveFormats"} {
		for _, item := range util.AsSlice(streaming[key]) {
			m := util.AsMap(item)
			if m == nil || util.AsString(m["url"]) == "" {
				continue
			}
			mime := util.AsString(m["mimeType"])
			video := strings.HasPrefix(mime, "video/")
			audio := strings.HasPrefix(mime, "audio/")
			if strings.Contains(mime, ";") {
				video = strings.HasPrefix(mime, "video/")
				audio = strings.HasPrefix(mime, "audio/")
			}
			formats = append(formats, model.Format{
				FormatID: util.AsString(m["itag"]), URL: util.AsString(m["url"]), Ext: youtubeExt(mime),
				Quality: util.FirstString(m, "qualityLabel", "quality"), Width: int(util.ToInt64(m["width"])), Height: int(util.ToInt64(m["height"])),
				Filesize: util.ToInt64(m["contentLength"]), VCodec: util.AsString(m["videoCodec"]), ACodec: util.AsString(m["audioCodec"]),
				HasVideo: video, HasAudio: audio, Headers: headers, Preference: int(util.ToInt64(m["bitrate"])),
			})
		}
	}
	sort.SliceStable(formats, func(i, j int) bool { return formatRank(formats[i]) > formatRank(formats[j]) })
	return formats
}

func youtubeExt(mime string) string {
	if strings.Contains(mime, "webm") {
		return "webm"
	}
	if strings.Contains(mime, "audio/") {
		return "m4a"
	}
	return "mp4"
}

func (e *Extractor) extractTwitter(client *httpx.Client, pageURL, id string) (*model.VideoInfo, error) {
	if id == "" {
		return nil, fmt.Errorf("无法解析 X/Twitter 推文 ID")
	}
	api := "https://cdn.syndication.twimg.com/tweet-result?id=" + url.QueryEscape(id) + "&lang=zh"
	body, resp, err := client.GetString(api, map[string]string{"Referer": pageURL, "Accept": "application/json"})
	if err != nil || resp.StatusCode >= http.StatusBadRequest {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("X/Twitter syndication HTTP %d", resp.StatusCode)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil, err
	}
	info := &model.VideoInfo{Platform: "twitter", ID: id, WebpageURL: pageURL}
	info.Title = firstNonEmpty(util.FirstString(data, "text"), util.FirstString(util.AsMap(data["legacy"]), "full_text"), id)
	info.Author = firstNonEmpty(util.FirstString(util.AsMap(data["user"]), "name"), util.FirstString(util.AsMap(data["core"]), "name"))
	info.Formats = twitterFormats(data, pageHeaders(e.config, pageURL))
	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("X/Twitter 推文没有可下载视频")
	}
	for _, item := range util.AsSlice(data["mediaDetails"]) {
		media := util.AsMap(item)
		if media != nil {
			info.CoverURL = firstNonEmpty(info.CoverURL, util.FirstString(media, "media_url_https", "media_url"))
		}
	}
	return info, nil
}

func twitterFormats(data map[string]any, headers map[string]string) []model.Format {
	var out []model.Format
	for _, item := range util.AsSlice(data["mediaDetails"]) {
		media := util.AsMap(item)
		video := util.AsMap(media["video_info"])
		for _, variant := range util.AsSlice(video["variants"]) {
			m := util.AsMap(variant)
			u := util.AsString(m["url"])
			if u == "" || !strings.Contains(util.AsString(m["content_type"]), "video") {
				continue
			}
			ext := "mp4"
			if strings.Contains(u, ".m3u8") {
				ext = "mp4"
			}
			out = append(out, model.Format{FormatID: "twitter_" + strconv.Itoa(len(out)+1), URL: u, Ext: ext, Quality: "", HasVideo: true, HasAudio: true, Headers: headers, Preference: int(util.ToInt64(m["bitrate"]))})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Preference > out[j].Preference })
	return out
}

func findCapture(body, pattern string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(body)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func matchYouTube(raw string) bool {
	return strings.Contains(raw, "youtube.com") || strings.Contains(raw, "youtu.be/")
}
func matchTikTok(raw string) bool { return strings.Contains(raw, "tiktok.com") }
func matchKuaishou(raw string) bool {
	return strings.Contains(raw, "kuaishou.com") || strings.Contains(raw, "kuaishouapp.com") || strings.Contains(raw, "kwai.com")
}
func matchBaidu(raw string) bool {
	return strings.Contains(raw, "haokan.baidu.com") || strings.Contains(raw, "video.baidu.com") || strings.Contains(raw, "v.baidu.com")
}
func matchTwitter(raw string) bool {
	return strings.Contains(raw, "t.co/") || ((strings.Contains(raw, "x.com/") || strings.Contains(raw, "twitter.com/")) && strings.Contains(raw, "/status/"))
}
func matchDouyu(raw string) bool { return strings.Contains(raw, "douyu.com") }
func matchHuya(raw string) bool  { return strings.Contains(raw, "huya.com") }

func youtubeID(raw string) string {
	patterns := []string{`(?i)[?&]v=([A-Za-z0-9_-]{6,})`, `(?i)youtu\.be/([A-Za-z0-9_-]{6,})`, `(?i)/shorts/([A-Za-z0-9_-]{6,})`, `(?i)/embed/([A-Za-z0-9_-]{6,})`}
	return firstRegex(raw, patterns...)
}

func twitterID(raw string) string { return firstRegex(raw, `(?i)/status/(\d{8,})`) }
func numericID(raw string) string { return firstRegex(raw, `/(\d{15,})`) }
func kuaishouID(raw string) string {
	return firstRegex(raw, `(?i)/(?:short-video|photo)/([A-Za-z0-9_-]+)`)
}

func baiduID(raw string) string {
	return firstRegex(raw, `(?i)[?&](?:vid|id)=([A-Za-z0-9_-]+)`, `(?i)/(?:video|v)/([A-Za-z0-9_-]+)`)
}

func douyuID(raw string) string {
	return firstRegex(raw, `(?i)douyu\.com/(?:room/)?([0-9]+)`, `(?i)v\.douyu\.com/([A-Za-z0-9_-]+)`)
}

func huyaID(raw string) string {
	return firstRegex(raw, `(?i)huya\.com/([A-Za-z0-9_-]+)`)
}

func genericID(raw string) string {
	return firstRegex(raw, `(?i)/(?:video|room|live|show|v)/([A-Za-z0-9_-]+)`)
}

func firstRegex(raw string, patterns ...string) string {
	for _, pattern := range patterns {
		if match := regexp.MustCompile(pattern).FindStringSubmatch(raw); len(match) > 1 {
			return match[1]
		}
	}
	return ""
}
