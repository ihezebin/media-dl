package weibo

import (
	"encoding/json"
	"fmt"
	"math/rand"
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

// 参考 yt-dlp weibo extractor：访客 Cookie + ajax/statuses/show；
// 微博 TV / H5 路径同时参考 lux。

var (
	statusRe = regexp.MustCompile(`(?i)(?:m\.weibo\.cn/(?:status|detail)|(?:www\.)?weibo\.com/\d+)/([A-Za-z0-9]+)`)
	tvOIDRe  = regexp.MustCompile(`(?i)(?:weibo\.com/tv/show/|fid=)(\d+:(?:[\da-f]{32}|\d{16,}))`)
	h5Re     = regexp.MustCompile(`(?i)video\.h5\.weibo\.cn/[^/]+/([^/?#]+)`)
	anyIDRe  = regexp.MustCompile(`(?i)/(?:status|detail)/([A-Za-z0-9]+)`)
)

type Extractor struct{}

func (e *Extractor) Name() string { return "weibo" }

func (e *Extractor) Match(rawURL string) bool {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = rawURL
	}
	lu := strings.ToLower(u)
	return strings.Contains(lu, "weibo.com") || strings.Contains(lu, "weibo.cn")
}

func (e *Extractor) Extract(client *httpx.Client, rawURL string) (*model.VideoInfo, error) {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = strings.TrimSpace(rawURL)
	}
	headers := map[string]string{
		"Referer":         "https://weibo.com/",
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
	}

	if m := tvOIDRe.FindStringSubmatch(u); len(m) > 1 {
		if info, err := extractTV(client, u, m[1], headers); err == nil {
			return info, nil
		}
	}
	if m := h5Re.FindStringSubmatch(u); len(m) > 1 {
		if info, err := extractH5(client, u, headers); err == nil {
			return info, nil
		}
	}

	id := parseStatusID(u)
	if id == "" {
		return nil, fmt.Errorf("无法解析微博 ID: %s", u)
	}

	status, err := fetchStatus(client, id, headers)
	if err != nil {
		return nil, err
	}
	return parseStatus(status, u)
}

func parseStatusID(rawURL string) string {
	if m := statusRe.FindStringSubmatch(rawURL); len(m) > 1 {
		return m[1]
	}
	if m := anyIDRe.FindStringSubmatch(rawURL); len(m) > 1 {
		return m[1]
	}
	return ""
}

func fetchStatus(client *httpx.Client, id string, headers map[string]string) (map[string]any, error) {
	var errs []string
	if data, err := fetchMobileShow(client, id, headers); err == nil {
		return data, nil
	} else {
		errs = append(errs, err.Error())
	}
	if data, err := fetchPCShow(client, id, headers); err == nil {
		return data, nil
	} else {
		errs = append(errs, err.Error())
	}
	return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
}

func fetchMobileShow(client *httpx.Client, id string, headers map[string]string) (map[string]any, error) {
	api := "https://m.weibo.cn/statuses/show?id=" + url.QueryEscape(id)
	h := map[string]string{
		"Referer": "https://m.weibo.cn/",
		"Accept":  "application/json, text/plain, */*",
	}
	for k, v := range headers {
		if k != "Referer" {
			h[k] = v
		}
	}
	body, resp, err := client.GetString(api, h)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("m.weibo.cn HTTP %d", resp.StatusCode)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil, fmt.Errorf("m.weibo.cn JSON 失败: %w", err)
	}
	if ok, _ := raw["ok"].(float64); ok != 1 && util.ToInt64(raw["ok"]) != 1 {
		if data := util.AsMap(raw["data"]); data != nil && data["page_info"] != nil {
			return data, nil
		}
		return nil, fmt.Errorf("m.weibo.cn ok=%v", raw["ok"])
	}
	data := util.AsMap(raw["data"])
	if data == nil {
		return nil, fmt.Errorf("m.weibo.cn 无 data")
	}
	return data, nil
}

func fetchPCShow(client *httpx.Client, id string, headers map[string]string) (map[string]any, error) {
	if err := ensureVisitor(client); err != nil {
		return nil, err
	}
	api := "https://weibo.com/ajax/statuses/show?id=" + url.QueryEscape(id)
	body, resp, err := weiboGet(client, api, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ajax/statuses/show HTTP %d", resp.StatusCode)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil, fmt.Errorf("ajax/statuses/show JSON 失败: %w", err)
	}
	if data["page_info"] == nil && data["mix_media_info"] == nil {
		return nil, fmt.Errorf("ajax/statuses/show 无媒体信息（可能需要 --cookies）")
	}
	return data, nil
}

func weiboGet(client *httpx.Client, api string, headers map[string]string) (string, *struct{ StatusCode int }, error) {
	body, resp, err := client.GetString(api, headers)
	if err != nil {
		return "", nil, err
	}
	if resp.Request != nil && strings.Contains(resp.Request.URL.Host, "passport.weibo.com") {
		if err := updateVisitor(client, resp.Request.URL.String()); err != nil {
			return "", nil, err
		}
		body, resp, err = client.GetString(api, headers)
		if err != nil {
			return "", nil, err
		}
	}
	return body, &struct{ StatusCode int }{resp.StatusCode}, nil
}

func ensureVisitor(client *httpx.Client) error {
	u, _ := url.Parse("https://weibo.com/")
	for _, c := range client.HTTP().Jar.Cookies(u) {
		if (c.Name == "SUB" || c.Name == "SUBP") && c.Value != "" {
			return nil
		}
	}
	return updateVisitor(client, "https://weibo.com/")
}

func updateVisitor(client *httpx.Client, visitorURL string) error {
	headers := map[string]string{"Referer": visitorURL}
	fp, _ := json.Marshal(map[string]string{
		"os":         "1",
		"browser":    "Chrome120,0,0,0",
		"fonts":      "undefined",
		"screenInfo": "1920*1080*24",
		"plugins":    "",
	})
	form := url.Values{}
	form.Set("cb", "gen_callback")
	form.Set("fp", string(fp))
	body, _, err := client.PostForm("https://passport.weibo.com/visitor/genvisitor", form, headers)
	if err != nil {
		return fmt.Errorf("生成微博访客 Cookie 失败: %w", err)
	}
	var wrap map[string]any
	if err := json.Unmarshal([]byte(util.StripJSONP(string(body))), &wrap); err != nil {
		return fmt.Errorf("解析 genvisitor 失败: %w", err)
	}
	data := util.AsMap(wrap["data"])
	if data == nil {
		return fmt.Errorf("genvisitor 无 data")
	}
	tid := util.AsString(data["tid"])
	if tid == "" {
		return fmt.Errorf("genvisitor 无 tid")
	}
	w := "2"
	if data["new_tid"] == true {
		w = "3"
	}
	c := fmt.Sprintf("%03d", util.ToInt64(data["confidence"]))
	if c == "000" {
		c = "100"
	}
	q := url.Values{}
	q.Set("a", "incarnate")
	q.Set("t", tid)
	q.Set("w", w)
	q.Set("c", c)
	q.Set("gc", "")
	q.Set("cb", "cross_domain")
	q.Set("from", "weibo")
	q.Set("_rand", fmt.Sprintf("%f", rand.Float64()))
	_, resp, err := client.GetString("https://passport.weibo.com/visitor/visitor?"+q.Encode(), headers)
	if err != nil {
		return fmt.Errorf("incarnate 访客 Cookie 失败: %w", err)
	}
	_ = resp
	return nil
}

func parseStatus(status map[string]any, pageURL string) (*model.VideoInfo, error) {
	id := util.FirstString(status, "id", "id_str", "mid")
	if id == "" || id == "<nil>" {
		id = fmt.Sprint(util.ToInt64(status["id"]))
	}
	info := &model.VideoInfo{
		Platform:    "weibo",
		ID:          id,
		WebpageURL:  pageURL,
		Title:       strings.ReplaceAll(util.AsString(status["status_title"]), "\n", " "),
		Description: util.FirstString(status, "text_raw", "text"),
	}
	if user := util.AsMap(status["user"]); user != nil {
		info.Author = util.FirstString(user, "screen_name", "name")
		info.AuthorID = util.FirstString(user, "idstr", "id")
		if info.AuthorID == "" {
			info.AuthorID = fmt.Sprint(util.ToInt64(user["id"]))
		}
	}

	media := util.AsMap(util.Nested(status, "page_info", "media_info"))
	if media == nil {
		if mix := util.AsMap(status["mix_media_info"]); mix != nil {
			if items, ok := mix["items"].([]any); ok {
				for _, it := range items {
					im := util.AsMap(it)
					if im == nil || util.FirstString(im, "type") == "pic" {
						continue
					}
					data := util.AsMap(im["data"])
					if data == nil {
						continue
					}
					media = util.AsMap(data["media_info"])
					if media != nil {
						if info.ID == "" {
							info.ID = util.AsString(data["object_id"])
						}
						break
					}
				}
			}
		}
	}
	pageInfo := util.AsMap(status["page_info"])
	if info.Title == "" && pageInfo != nil {
		info.Title = util.FirstString(util.AsMap(pageInfo["media_info"]), "video_title", "kol_title", "name")
		if info.Title == "" {
			info.Title = util.FirstString(pageInfo, "page_title", "title")
		}
	}
	if info.Title == "" {
		info.Title = id
	}
	if media != nil {
		info.Duration = util.ToFloat(media["duration"])
		if pageInfo != nil {
			info.CoverURL = util.FirstString(util.AsMap(pageInfo["page_pic"]), "url")
			if info.CoverURL == "" {
				info.CoverURL = util.AsString(pageInfo["page_pic"])
			}
		}
		info.Formats = extractPlayback(media)
	}
	if len(info.Formats) == 0 && pageInfo != nil {
		if urls := util.AsMap(pageInfo["urls"]); urls != nil {
			info.Formats = urlsToFormats(urls)
		}
	}
	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("未找到可下载视频: %s", id)
	}
	return info, nil
}

func extractPlayback(media map[string]any) []model.Format {
	headers := map[string]string{"Referer": "https://weibo.com/"}
	var out []model.Format
	if list, ok := media["playback_list"].([]any); ok {
		for _, item := range list {
			pi := util.AsMap(util.Nested(item, "play_info"))
			if pi == nil {
				continue
			}
			u := util.FirstString(pi, "url")
			if u == "" {
				continue
			}
			h := int(util.ToInt64(pi["height"]))
			out = append(out, model.Format{
				FormatID:   util.FirstString(pi, "label", "quality_desc"),
				URL:        u,
				Ext:        "mp4",
				Width:      int(util.ToInt64(pi["width"])),
				Height:     h,
				Filesize:   util.ToInt64(pi["size"]),
				VCodec:     util.AsString(pi["video_codecs"]),
				ACodec:     util.AsString(pi["audio_codecs"]),
				HasVideo:   true,
				HasAudio:   true,
				Quality:    util.FirstString(pi, "quality_desc", "label"),
				Preference: h,
				Headers:    headers,
			})
		}
	}
	if len(out) == 0 {
		for _, key := range []string{"stream_url_hd", "stream_url", "mp4_hd_url", "mp4_sd_url"} {
			if u := util.AsString(media[key]); u != "" {
				out = append(out, model.Format{
					FormatID:   key,
					URL:        u,
					Ext:        "mp4",
					HasVideo:   true,
					HasAudio:   true,
					Preference: 10,
					Headers:    headers,
				})
			}
		}
	}
	return out
}

func urlsToFormats(urls map[string]any) []model.Format {
	headers := map[string]string{"Referer": "https://weibo.com/"}
	var out []model.Format
	for k, v := range urls {
		u, ok := v.(string)
		if !ok || u == "" {
			continue
		}
		pref := 10
		lk := strings.ToLower(k)
		switch {
		case strings.Contains(lk, "1080"):
			pref = 1080
		case strings.Contains(lk, "720"):
			pref = 720
		case strings.Contains(lk, "480"):
			pref = 480
		}
		out = append(out, model.Format{
			FormatID:   k,
			URL:        u,
			Ext:        "mp4",
			HasVideo:   true,
			HasAudio:   true,
			Quality:    k,
			Preference: pref,
			Headers:    headers,
		})
	}
	return out
}

func extractTV(client *httpx.Client, pageURL, oid string, headers map[string]string) (*model.VideoInfo, error) {
	if err := ensureVisitor(client); err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("data", fmt.Sprintf(`{"Component_Play_Playinfo":{"oid":%q}}`, oid))
	h := map[string]string{
		"Referer":      pageURL,
		"Content-Type": "application/x-www-form-urlencoded",
	}
	api := "https://weibo.com/tv/api/component?page=" + url.QueryEscape("/tv/show/"+oid)
	body, resp, err := client.PostForm(api, form, h)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tv/api HTTP %d", resp.StatusCode)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	play := util.AsMap(util.Nested(raw, "data", "Component_Play_Playinfo"))
	if play == nil {
		return nil, fmt.Errorf("tv/api 无 Playinfo")
	}
	if mid := util.AsString(play["mid"]); mid != "" {
		if status, err := fetchStatus(client, mid, headers); err == nil {
			if info, err := parseStatus(status, pageURL); err == nil {
				return info, nil
			}
		}
	}
	info := &model.VideoInfo{
		Platform:   "weibo",
		ID:         oid,
		Title:      util.FirstString(play, "title", "name"),
		Author:     util.AsString(play["author"]),
		CoverURL:   absURL(util.AsString(play["cover_image"])),
		WebpageURL: pageURL,
		Duration:   util.ToFloat(play["duration"]),
	}
	if info.Title == "" {
		info.Title = oid
	}
	if urls := util.AsMap(play["urls"]); urls != nil {
		fixed := map[string]any{}
		for k, v := range urls {
			if s, ok := v.(string); ok {
				fixed[k] = absURL(s)
			}
		}
		info.Formats = urlsToFormats(fixed)
	}
	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("微博 TV 无播放地址: %s", oid)
	}
	return info, nil
}

func extractH5(client *httpx.Client, pageURL string, headers map[string]string) (*model.VideoInfo, error) {
	pu, err := url.Parse(pageURL)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.Trim(pu.Path, "/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("无法解析 H5 视频路径")
	}
	api := fmt.Sprintf("https://video.h5.weibo.cn/s/video/object?object_id=%s&mid=%s",
		url.QueryEscape(parts[0]), url.QueryEscape(parts[1]))
	body, resp, err := client.GetString(api, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("h5 video HTTP %d", resp.StatusCode)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil, err
	}
	obj := util.AsMap(util.Nested(raw, "data", "object", "object"))
	if obj == nil {
		obj = util.AsMap(util.Nested(raw, "data", "object"))
	}
	stream := util.AsMap(obj["stream"])
	info := &model.VideoInfo{
		Platform:   "weibo",
		ID:         parts[len(parts)-1],
		Title:      util.FirstString(obj, "summary", "title"),
		CoverURL:   util.AsString(obj["image"]),
		WebpageURL: pageURL,
		Duration:   util.ToFloat(stream["duration"]),
	}
	if info.Title == "" {
		info.Title = info.ID
	}
	headersDL := map[string]string{"Referer": "https://weibo.com/"}
	for _, key := range []string{"hd_url", "url"} {
		u := strings.ReplaceAll(util.AsString(stream[key]), `\/`, `/`)
		if u == "" {
			continue
		}
		pref := 720
		if key == "url" {
			pref = 360
		}
		info.Formats = append(info.Formats, model.Format{
			FormatID:   key,
			URL:        u,
			Ext:        "mp4",
			HasVideo:   true,
			HasAudio:   true,
			Preference: pref,
			Headers:    headersDL,
		})
	}
	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("H5 接口无播放地址")
	}
	return info, nil
}

func absURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	return u
}
