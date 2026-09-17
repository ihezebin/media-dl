// Package web contains extractors for platforms whose playable media is
// exposed by the public page bootstrap data or a small public metadata API.
package web

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
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
	{name: "tiktok", match: matchTikTok, id: numericID, host: "https://www.tiktok.com/", pageUA: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 Version/18.6 Mobile/15E148 Safari/604.1"},
	{name: "kuaishou", match: matchKuaishou, id: kuaishouID, host: "https://www.kuaishou.com/", pageUA: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 Version/18.6 Mobile/15E148 Safari/604.1"},
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
	if e.config.name == "kuaishou" {
		if kuaishouInfo := parseKuaishouPage(body, pageURL, id, headers); kuaishouInfo != nil {
			return kuaishouInfo, nil
		}
	}
	if e.config.name == "huya" {
		if huyaInfo := parseHuyaPage(body, pageURL, id, headers); huyaInfo != nil {
			return huyaInfo, nil
		}
	}
	if e.config.name == "douyu" {
		return e.extractDouyu(client, body, pageURL, id, headers)
	}
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
	referer := pageURL
	// 快手会在带当前短视频 URL 作为 Referer 的请求中返回只有站点配置的
	// shell；使用站点首页作为 Referer 才会下发当前页面的 Apollo 状态。
	if p.name == "kuaishou" {
		referer = p.host
	}
	return map[string]string{
		"User-Agent":      ua,
		"Referer":         referer,
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

func parseKuaishouPage(body, pageURL, id string, headers map[string]string) *model.VideoInfo {
	var photo map[string]any
	var root map[string]any
	for _, name := range []string{"__APOLLO_STATE__", "INIT_STATE"} {
		value, ok := embeddedJSONObject(body, name)
		if !ok {
			continue
		}
		if root == nil {
			root = value
		}
		if photo == nil {
			photo = findKuaishouPhoto(value, id)
		}
	}
	if photo == nil {
		return nil
	}

	info := &model.VideoInfo{
		Platform:   "kuaishou",
		ID:         firstNonEmpty(util.AsString(photo["id"]), id),
		Title:      util.AsString(photo["caption"]),
		CoverURL:   firstNonEmpty(util.AsString(photo["coverUrl"]), util.AsString(photo["poster"])),
		WebpageURL: pageURL,
	}
	if duration := util.ToInt64(photo["duration"]); duration > 0 {
		info.Duration = float64(duration) / 1000
	}
	if author := findKuaishouAuthor(root); author != nil {
		info.Author = firstNonEmpty(util.AsString(author["name"]), util.AsString(author["screen_name"]))
		info.AuthorID = firstNonEmpty(util.AsString(author["id"]), util.AsString(author["userId"]))
	}

	var candidates []mediaCandidate
	for _, key := range []string{"photoUrl", "photoH265Url", "croppedPhotoUrl", "croppedPhotoH265Url"} {
		if value := mediaURL(util.AsString(photo[key])); value != "" {
			candidates = append(candidates, mediaCandidate{url: value, key: key})
		}
	}
	collectJSONCandidates(photo, &candidates)
	info.Formats = formatsFromCandidates(candidates, headers)
	if len(info.Formats) == 0 {
		return nil
	}
	if info.Title == "" {
		info.Title = info.ID
	}
	return info
}

func parseHuyaPage(body, pageURL, id string, headers map[string]string) *model.VideoInfo {
	// 虎牙直播页把带 anti-code 的播放参数放在 stream.gameStreamInfoList 中，
	// sFlvUrl 本身只是 /src 基地址，必须拼上 sStreamName 才是可请求的媒体 URL。
	streamRe := regexp.MustCompile(`"sStreamName":"([^"]+)"\s*,"sFlvUrl":"([^"]+)"\s*,"sFlvUrlSuffix":"([^"]*)"\s*,"sFlvAntiCode":"([^"]*)"`)
	matches := streamRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	info := pageInfo("huya", id, pageURL, body)
	for _, match := range matches {
		if len(match) < 5 {
			continue
		}
		base := strings.TrimRight(mediaURL(match[2]), "/")
		stream := mediaURL("https://placeholder.invalid/" + match[1])
		if parsed, err := url.Parse(stream); err == nil {
			stream = strings.TrimPrefix(parsed.Path, "/")
		}
		antiCode := strings.TrimSpace(match[4])
		if base == "" || stream == "" {
			continue
		}
		streamURL := base + "/" + stream
		if antiCode != "" {
			streamURL += "?" + antiCode
		}
		info.Formats = append(info.Formats, model.Format{
			FormatID: "flv_" + strconv.Itoa(len(info.Formats)+1), URL: streamURL, Ext: "flv",
			Quality: "直播流", HasVideo: true, HasAudio: true, Headers: headers,
		})
	}
	if len(info.Formats) == 0 {
		return nil
	}
	return info
}

func (e *Extractor) extractDouyu(client *httpx.Client, body, pageURL, id string, headers map[string]string) (*model.VideoInfo, error) {
	if id == "" {
		return nil, fmt.Errorf("无法解析斗鱼房间 ID")
	}
	info := pageInfo("douyu", id, pageURL, body)
	roomName := firstNonEmpty(findCapture(body, `"room_name":"([^"]+)"`), findCapture(body, `roomName[^>]*>([^<]+)<`))
	info.Title = firstNonEmpty(roomName, info.Title, id)
	info.CoverURL = firstNonEmpty(info.CoverURL, findCapture(body, `"coverSrc":"([^"]+)"`))

	stream, err := douyuLiveStream(client, id, pageURL, headers)
	if err != nil {
		return nil, err
	}
	info.Formats = []model.Format{{
		FormatID: "douyu_live", URL: stream, Ext: "flv", Quality: "直播流",
		HasVideo: true, HasAudio: true, Headers: headers,
	}}
	return info, nil
}

func douyuLiveStream(client *httpx.Client, roomID, pageURL string, headers map[string]string) (string, error) {
	encURL := "https://www.douyu.com/swf_api/homeH5Enc?rids=" + url.QueryEscape(roomID)
	encBody, encResp, err := client.GetString(encURL, headers)
	if err != nil {
		return "", fmt.Errorf("请求斗鱼播放签名脚本失败: %w", err)
	}
	if encResp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("斗鱼播放签名脚本 HTTP %d", encResp.StatusCode)
	}
	var encoded struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal([]byte(encBody), &encoded); err != nil {
		return "", fmt.Errorf("解析斗鱼播放签名脚本失败: %w", err)
	}
	script := encoded.Data["room"+roomID]
	if script == "" {
		return "", fmt.Errorf("斗鱼未返回房间播放签名脚本")
	}

	did := fmt.Sprintf("%x", md5.Sum([]byte(roomID+time.Now().UTC().Format(time.RFC3339Nano))))
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	vm := goja.New()
	cryptoJS := vm.NewObject()
	if err := cryptoJS.Set("MD5", func(call goja.FunctionCall) goja.Value {
		digest := fmt.Sprintf("%x", md5.Sum([]byte(call.Argument(0).String())))
		result := vm.NewObject()
		_ = result.Set("toString", func(goja.FunctionCall) goja.Value { return vm.ToValue(digest) })
		return result
	}); err != nil {
		return "", fmt.Errorf("初始化斗鱼签名环境失败: %w", err)
	}
	if err := vm.Set("CryptoJS", cryptoJS); err != nil {
		return "", fmt.Errorf("初始化斗鱼签名环境失败: %w", err)
	}
	value, err := vm.RunString(script + `;ub98484234`)
	if err != nil {
		return "", fmt.Errorf("执行斗鱼播放签名失败: %w", err)
	}
	sign, ok := goja.AssertFunction(value)
	if !ok {
		return "", fmt.Errorf("斗鱼播放签名函数不存在")
	}
	result, err := sign(goja.Undefined(), vm.ToValue(roomID), vm.ToValue(did), vm.ToValue(timestamp))
	if err != nil {
		return "", fmt.Errorf("计算斗鱼播放签名失败: %w", err)
	}
	query := result.String()
	query += "&cdn=tct-h5&rate=0"
	playURL := "https://www.douyu.com/lapi/live/getH5Play/" + url.PathEscape(roomID) + "?" + query
	playBody, playResp, err := client.PostBytes(playURL, "", nil, map[string]string{
		"Origin": "https://www.douyu.com", "Referer": pageURL, "Accept": "application/json",
	})
	if err != nil {
		return "", fmt.Errorf("请求斗鱼播放地址失败: %w", err)
	}
	if playResp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("斗鱼播放地址 HTTP %d", playResp.StatusCode)
	}
	var payload struct {
		Error int    `json:"error"`
		Msg   string `json:"msg"`
		Data  struct {
			RTMPURL  string `json:"rtmp_url"`
			RTMPLive string `json:"rtmp_live"`
			Online   int    `json:"online"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(playBody), &payload); err != nil {
		return "", fmt.Errorf("解析斗鱼播放地址失败: %w", err)
	}
	if payload.Error != 0 || payload.Data.RTMPURL == "" || payload.Data.RTMPLive == "" {
		if payload.Msg == "" {
			payload.Msg = "直播间未开播或平台拒绝了播放请求"
		}
		return "", fmt.Errorf("斗鱼没有可用播放地址: %s", payload.Msg)
	}
	return strings.TrimRight(payload.Data.RTMPURL, "/") + "/" + strings.TrimLeft(payload.Data.RTMPLive, "/"), nil
}

func findKuaishouPhoto(value any, id string) map[string]any {
	return findKuaishouMap(value, func(item map[string]any) bool {
		typename := strings.ToLower(util.AsString(item["__typename"]))
		return typename == "visionvideodetailphoto" || (id != "" && util.AsString(item["id"]) == id && item["photoUrl"] != nil)
	})
}

func findKuaishouAuthor(value any) map[string]any {
	return findKuaishouMap(value, func(item map[string]any) bool {
		typename := strings.ToLower(util.AsString(item["__typename"]))
		return strings.Contains(typename, "author") && util.AsString(item["name"]) != ""
	})
}

func findKuaishouMap(value any, match func(map[string]any) bool) map[string]any {
	switch item := value.(type) {
	case map[string]any:
		if match(item) {
			return item
		}
		for _, child := range item {
			if found := findKuaishouMap(child, match); found != nil {
				return found
			}
		}
	case []any:
		for _, child := range item {
			if found := findKuaishouMap(child, match); found != nil {
				return found
			}
		}
	}
	return nil
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
	mediaAttrRe := regexp.MustCompile(`(?is)(?:href|src)=["'](https?://[^"']+\.(?:mp4|m3u8|flv)(?:\?[^"']*)?)["']`)
	for _, match := range mediaAttrRe.FindAllStringSubmatch(body, -1) {
		if len(match) > 1 {
			if u := mediaURL(match[1]); u != "" {
				candidates = append(candidates, mediaCandidate{url: u, key: "page_media"})
			}
		}
	}
	for _, block := range jsonLDBlocks(body) {
		var value any
		if json.Unmarshal([]byte(block), &value) == nil {
			collectJSONCandidates(value, &candidates)
		}
	}
	for _, name := range []string{"__APOLLO_STATE__", "INIT_STATE"} {
		if value, ok := embeddedJSONObject(body, name); ok {
			collectJSONCandidates(value, &candidates)
		}
	}
	if value, ok := embeddedScriptJSON(body, "__UNIVERSAL_DATA_FOR_REHYDRATION__"); ok {
		collectJSONCandidates(value, &candidates)
	}
	return formatsFromCandidates(candidates, headers)
}

func formatsFromCandidates(candidates []mediaCandidate, headers map[string]string) []model.Format {

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
	return key == "url" || key == "backupurl" || strings.Contains(key, "video") || strings.Contains(key, "playaddr") || strings.Contains(key, "playurl") || strings.Contains(key, "downloadaddr") || strings.Contains(key, "contenturl") || strings.Contains(key, "photourl") || strings.Contains(key, "m3u8") || strings.Contains(key, "streamurl") || strings.Contains(key, "hls") || strings.Contains(key, "flv") || strings.Contains(key, "rtmp")
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
	if strings.Contains(key, "video") || strings.Contains(key, "play") || strings.Contains(key, "stream") || strings.Contains(key, "content") || strings.Contains(key, "photo") || strings.Contains(key, "hls") || strings.Contains(key, "flv") || strings.Contains(key, "rtmp") {
		return true
	}
	return strings.Contains(low, ".mp4") || strings.Contains(low, ".m3u8") || strings.Contains(low, ".webm") || strings.Contains(low, ".m4s") || strings.Contains(low, "mime=video")
}

func embeddedJSONObject(body, name string) (map[string]any, bool) {
	marker := "window." + name
	start := strings.Index(body, marker)
	if start < 0 {
		return nil, false
	}
	start += len(marker)
	assign := strings.Index(body[start:], "{")
	if assign < 0 {
		return nil, false
	}
	start += assign
	end := jsonObjectEnd(body, start)
	if end <= start {
		return nil, false
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(body[start:end]), &value); err != nil {
		return nil, false
	}
	return value, true
}

func embeddedScriptJSON(body, id string) (map[string]any, bool) {
	re := regexp.MustCompile(`(?is)<script[^>]+id=["']` + regexp.QuoteMeta(id) + `["'][^>]*>(.*?)</script>`)
	match := re.FindStringSubmatch(body)
	if len(match) < 2 {
		return nil, false
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(match[1])), &value); err != nil {
		return nil, false
	}
	return value, true
}

func jsonObjectEnd(body string, start int) int {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(body); i++ {
		ch := body[i]
		if inString {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
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
	api := "https://www.youtube.com/youtubei/v1/player?key=" + url.QueryEscape(key)
	// WEB 在部分网络/版本组合下只返回 signatureCipher 或 UNPLAYABLE；
	// ANDROID 客户端仍会返回可直接请求的带签名 URL。优先 WEB，缺少直链时回退。
	clients := []struct{ name, version string }{
		{"WEB", version},
		{"ANDROID", "20.10.38"},
	}
	var data map[string]any
	var lastErr error
	for _, ytClient := range clients {
		requestBody, _ := json.Marshal(map[string]any{
			"context": map[string]any{"client": map[string]string{"clientName": ytClient.name, "clientVersion": ytClient.version, "hl": "zh-CN", "gl": "US"}},
			"videoId": id,
		})
		apiBody, apiResp, requestErr := client.PostBytes(api, "application/json", requestBody, map[string]string{
			"Origin": "https://www.youtube.com", "Referer": pageURL, "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		})
		if requestErr != nil {
			lastErr = requestErr
			continue
		}
		if apiResp.StatusCode >= http.StatusBadRequest {
			lastErr = fmt.Errorf("YouTube 播放信息 HTTP %d", apiResp.StatusCode)
			continue
		}
		var candidate map[string]any
		if unmarshalErr := json.Unmarshal(apiBody, &candidate); unmarshalErr != nil {
			lastErr = fmt.Errorf("解析 YouTube 播放信息失败: %w", unmarshalErr)
			continue
		}
		data = candidate
		if len(youtubeFormats(data, pageHeaders(e.config, pageURL))) > 0 {
			break
		}
	}
	if data == nil {
		if lastErr != nil {
			return nil, fmt.Errorf("请求 YouTube 播放信息失败: %w", lastErr)
		}
		return nil, fmt.Errorf("请求 YouTube 播放信息失败")
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
			vcodec := util.AsString(m["videoCodec"])
			acodec := util.AsString(m["audioCodec"])
			audio := strings.HasPrefix(mime, "audio/") || acodec != "" || strings.Contains(mime, "mp4a") || strings.Contains(mime, "opus")
			if strings.Contains(mime, ";") {
				video = strings.HasPrefix(mime, "video/")
				audio = audio || strings.Contains(mime, "mp4a") || strings.Contains(mime, "opus")
			}
			formats = append(formats, model.Format{
				FormatID: util.AsString(m["itag"]), URL: util.AsString(m["url"]), Ext: youtubeExt(mime),
				Quality: util.FirstString(m, "qualityLabel", "quality"), Width: int(util.ToInt64(m["width"])), Height: int(util.ToInt64(m["height"])),
				Filesize: util.ToInt64(m["contentLength"]), VCodec: vcodec, ACodec: acodec,
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
	// Syndication API 要求带 token 查询参数；当前公开接口接受 token=0。
	// 不带该参数时会返回 200 + {}，看起来像成功但不会包含 mediaDetails。
	api := twitterSyndicationURL(id)
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

func twitterSyndicationURL(id string) string {
	query := url.Values{}
	query.Set("id", id)
	query.Set("lang", "zh")
	query.Set("token", "0")
	return "https://cdn.syndication.twimg.com/tweet-result?" + query.Encode()
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
