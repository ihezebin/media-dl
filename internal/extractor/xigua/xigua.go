package xigua

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/hezebin/media-dl/internal/extractor"
	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/model"
	"github.com/hezebin/media-dl/internal/util"
)

func init() {
	extractor.Register(&Extractor{})
}

// 参考 yt-dlp ixigua：PC 页有 antibot 时走头条 info + 火山 GetPlayInfo；有 Cookie 时可回退 SSR。

var (
	idRe             = regexp.MustCompile(`(?i)(?:ixigua\.com/(?:video/)?|toutiao\.com/(?:video/|a|group/|i)|365yg\.com/i)(\d{15,})`)
	shortURLRe       = regexp.MustCompile(`(?i)https?://(?:v\.ixigua\.com|m\.toutiao\.com)/[\w/-]+`)
	ssrRe            = regexp.MustCompile(`(?s)window\._SSR_HYDRATED_DATA\s*=`)
	tokenInContentRe = regexp.MustCompile(`data-token=['"]([^'"]+)['"]`)
)

type Extractor struct{}

func (e *Extractor) Name() string { return "xigua" }

func (e *Extractor) Match(rawURL string) bool {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = rawURL
	}
	lu := strings.ToLower(u)
	return strings.Contains(lu, "ixigua.com") ||
		strings.Contains(lu, "toutiao.com") ||
		strings.Contains(lu, "365yg.com")
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
	vid := ""
	if m := idRe.FindStringSubmatch(pageURL); len(m) > 1 {
		vid = m[1]
	}
	if vid == "" {
		return nil, fmt.Errorf("无法解析西瓜视频 ID: %s", pageURL)
	}
	_ = ensureTTWid(client)

	page := fmt.Sprintf("https://www.ixigua.com/%s", vid)
	if info, err := extractFromToutiaoInfo(client, vid, page); err == nil && info != nil && len(info.Formats) > 0 {
		return info, nil
	}

	headers := map[string]string{
		"Referer":         "https://www.ixigua.com/",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9",
	}
	body, resp, err := client.GetString(page, headers)
	if err != nil {
		return nil, fmt.Errorf("请求西瓜视频页失败: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("西瓜视频页 HTTP %d", resp.StatusCode)
	}

	idx := ssrRe.FindStringIndex(body)
	if idx == nil {
		return nil, fmt.Errorf("未找到播放地址（页面 antibot，可尝试 --cookies）")
	}
	raw, err := util.ExtractBalancedJSON(body, idx[1])
	if err != nil {
		return nil, fmt.Errorf("解析 SSR JSON 失败: %w", err)
	}
	cleaned := strings.ReplaceAll(string(raw), "undefined", "null")
	var state map[string]any
	if err := json.Unmarshal([]byte(cleaned), &state); err != nil {
		return nil, fmt.Errorf("SSR JSON 反序列化失败: %w", err)
	}

	video := findVideo(state)
	if video == nil {
		return nil, fmt.Errorf("SSR 中无视频数据: %s", vid)
	}

	info := &model.VideoInfo{
		Platform:    "xigua",
		ID:          vid,
		Title:       util.FirstString(video, "title"),
		Description: util.FirstString(video, "video_abstract"),
		Duration:    util.ToFloat(video["duration"]),
		CoverURL:    util.FirstString(video, "poster_url"),
		WebpageURL:  page,
	}
	if user := util.AsMap(video["user_info"]); user != nil {
		info.Author = util.FirstString(user, "name")
		info.AuthorID = util.FirstString(user, "user_id")
	}
	if info.Title == "" {
		info.Title = vid
	}

	info.Formats = extractFormats(util.AsMap(video["videoResource"]))
	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("未找到可下载媒体: %s", vid)
	}
	return info, nil
}

func extractFromToutiaoInfo(client *httpx.Client, groupID, page string) (*model.VideoInfo, error) {
	headers := map[string]string{
		"Referer": "https://www.ixigua.com/",
		"Accept":  "application/json,text/plain,*/*",
	}
	var data map[string]any
	var lastErr error
	for _, api := range []string{
		fmt.Sprintf("https://m.toutiao.com/i%s/info/", groupID),
		fmt.Sprintf("https://m.ixigua.com/i%s/info/", groupID),
	} {
		body, resp, err := client.GetBytes(api, headers)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("info HTTP %d", resp.StatusCode)
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			lastErr = err
			continue
		}
		data = util.AsMap(raw["data"])
		if data != nil {
			break
		}
		lastErr = fmt.Errorf("info 无 data")
	}
	if data == nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("头条 info 失败")
		}
		return nil, lastErr
	}

	info := &model.VideoInfo{
		Platform:   "xigua",
		ID:         groupID,
		Title:      util.FirstString(data, "title"),
		CoverURL:   util.FirstString(data, "poster_url"),
		Duration:   util.ToFloat(data["video_duration"]),
		WebpageURL: page,
	}
	if user := util.AsMap(data["media_user"]); user != nil {
		info.Author = util.FirstString(user, "screen_name", "name")
		if info.AuthorID == "" {
			if id := util.ToInt64(user["id"]); id > 0 {
				info.AuthorID = strconv.FormatInt(id, 10)
			} else {
				info.AuthorID = util.FirstString(user, "user_id")
			}
		}
	}
	if info.Title == "" {
		info.Title = groupID
	}

	token := util.FirstString(data, "play_auth_token_v2")
	if token == "" {
		if content := util.FirstString(data, "content"); content != "" {
			if m := tokenInContentRe.FindStringSubmatch(content); len(m) > 1 {
				token = m[1]
			}
		}
	}
	if token == "" {
		return nil, fmt.Errorf("无 play_auth_token")
	}
	fmts, err := fetchPlayInfo(client, token)
	if err != nil {
		return nil, err
	}
	info.Formats = fmts
	return info, nil
}

func fetchPlayInfo(client *httpx.Client, token string) ([]model.Format, error) {
	rawTok, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		rawTok, err = base64.RawStdEncoding.DecodeString(token)
	}
	if err != nil {
		return nil, fmt.Errorf("解码 play token 失败: %w", err)
	}
	var wrap map[string]any
	if err := json.Unmarshal(rawTok, &wrap); err != nil {
		return nil, fmt.Errorf("解析 play token 失败: %w", err)
	}
	q := util.FirstString(wrap, "GetPlayInfoToken")
	if q == "" {
		return nil, fmt.Errorf("play token 无 GetPlayInfoToken")
	}
	api := "https://vod.bytedanceapi.com/?" + q
	body, resp, err := client.GetBytes(api, map[string]string{
		"Referer": "https://www.ixigua.com/",
		"Accept":  "application/json",
	})
	if err != nil {
		return nil, fmt.Errorf("GetPlayInfo 失败: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GetPlayInfo HTTP %d", resp.StatusCode)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	data := util.AsMap(util.Nested(raw, "Result", "Data"))
	if data == nil {
		data = util.AsMap(raw["Result"])
	}
	if data == nil {
		return nil, fmt.Errorf("GetPlayInfo 无 Result")
	}
	list := util.AsSlice(data["PlayInfoList"])
	headers := map[string]string{"Referer": "https://www.ixigua.com/"}
	var out []model.Format
	for _, item := range list {
		m := util.AsMap(item)
		u := util.FirstString(m, "MainPlayUrl", "BackupPlayUrl")
		if u == "" {
			continue
		}
		h := int(util.ToInt64(m["Height"]))
		def := util.FirstString(m, "Definition", "Quality")
		ext := strings.ToLower(util.FirstString(m, "Format"))
		if ext == "" {
			ext = "mp4"
		}
		out = append(out, model.Format{
			FormatID:   def,
			URL:        u,
			Ext:        ext,
			Width:      int(util.ToInt64(m["Width"])),
			Height:     h,
			Filesize:   util.ToInt64(m["Size"]),
			VCodec:     util.FirstString(m, "Codec"),
			HasVideo:   true,
			HasAudio:   true,
			Quality:    def,
			Preference: h,
			Headers:    headers,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("GetPlayInfo 无播放地址")
	}
	return out, nil
}

func resolveURL(client *httpx.Client, rawURL string) (string, error) {
	if shortURLRe.MatchString(rawURL) {
		final, err := client.ResolveRedirect(rawURL, map[string]string{
			"Referer": "https://www.ixigua.com/",
		})
		if err != nil {
			return "", fmt.Errorf("解析西瓜短链失败: %w", err)
		}
		rawURL = final
	}
	rawURL = strings.ReplaceAll(rawURL, "https://www.toutiao.com/video/", "https://www.ixigua.com/")
	rawURL = strings.ReplaceAll(rawURL, "https://www.toutiao.com/a", "https://www.ixigua.com/")
	return rawURL, nil
}

func findVideo(state map[string]any) map[string]any {
	anyVideo := util.AsMap(state["anyVideo"])
	gid := util.AsMap(anyVideo["gidInformation"])
	packer := util.AsMap(gid["packerData"])
	if v := util.AsMap(packer["video"]); v != nil {
		return v
	}
	return util.AsMap(packer["videoData"])
}

func extractFormats(resource map[string]any) []model.Format {
	if resource == nil {
		return nil
	}
	headers := map[string]string{"Referer": "https://www.ixigua.com/"}
	var out []model.Format

	addList := func(list any, audioURL string, muxed bool) {
		switch t := list.(type) {
		case map[string]any:
			for _, v := range t {
				if f, ok := mediaFormat(util.AsMap(v), audioURL, muxed, headers); ok {
					out = append(out, f)
				}
			}
		case []any:
			for _, v := range t {
				if f, ok := mediaFormat(util.AsMap(v), audioURL, muxed, headers); ok {
					out = append(out, f)
				}
			}
		}
	}

	for _, packName := range []string{"normal", "dash", "dash_120fps"} {
		pack := util.AsMap(resource[packName])
		if pack == nil {
			continue
		}
		if dyn := util.AsMap(pack["dynamic_video"]); dyn != nil {
			audioURL := ""
			if auds, ok := dyn["dynamic_audio_list"].([]any); ok && len(auds) > 0 {
				audioURL = decodeMainURL(util.AsMap(auds[0]))
			}
			addList(dyn["dynamic_video_list"], audioURL, false)
		}
		addList(pack["video_list"], "", true)
	}
	return out
}

func mediaFormat(m map[string]any, audioURL string, muxed bool, headers map[string]string) (model.Format, bool) {
	u := decodeMainURL(m)
	if u == "" {
		return model.Format{}, false
	}
	h := int(util.ToInt64(m["vheight"]))
	def := util.FirstString(m, "definition", "quality")
	f := model.Format{
		FormatID:   util.FirstString(m, "quality_type", "definition"),
		URL:        u,
		Ext:        util.FirstString(m, "vtype"),
		Width:      int(util.ToInt64(m["vwidth"])),
		Height:     h,
		Filesize:   util.ToInt64(m["size"]),
		VCodec:     util.AsString(m["codec_type"]),
		HasVideo:   true,
		HasAudio:   muxed || audioURL != "",
		Quality:    def,
		Preference: h,
		Headers:    headers,
	}
	if f.Ext == "" {
		f.Ext = "mp4"
	}
	if !muxed {
		f.ACodec = "none"
		f.AudioURL = audioURL
		f.Preference = h - 1
	}
	return f, true
}

func decodeMainURL(m map[string]any) string {
	s := util.FirstString(m, "main_url", "backup_url_1")
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "http") {
		return s
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(b)
}

func ensureTTWid(client *httpx.Client) error {
	u, _ := url.Parse("https://www.ixigua.com/")
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
		return fmt.Errorf("未拿到 ttwid（可尝试 --cookies）")
	}
	for _, host := range []string{"www.ixigua.com", "www.toutiao.com"} {
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
