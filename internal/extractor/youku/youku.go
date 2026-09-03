package youku

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hezebin/media-dl/internal/extractor"
	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/model"
	"github.com/hezebin/media-dl/internal/util"
)

func init() {
	extractor.Register(&Extractor{})
}

// 参考 yt-dlp / you-get / lux：ups.youku.com/ups/get.json

const (
	youkuCKey = "DIl58SLFxFNndSV1GFNnMQVYkx1PP5tKe1siZu/86PR1u/Wh1Ptd+WOZsHHWxysSfAOhNJpdVWsdVJNsfJ8Sxd8WKVvNfAS8aS8fAOzYARzPyPc3JvtnPHjTdKfESTdnuTW6ZPvk2pNDh4uFzotgdMEFkzQ5wZVXl2Pf1/Y6hLK0OnCNxBj3+nb0v72gZ6b0td+WOZsHHWxysSo/0y9D2K42SaB8Y/+aD2K42SaB8Y/+ahU+WOZsHcrxysooUeND"
	luxCKey   = "7B19C0AB12633B22E7FE81271162026020570708D6CC189E4924503C49D243A0DE6CD84A766832C2C99898FC5ED31F3709BB3CDD82C96492E721BDD381735026"
)

var (
	vidRe = regexp.MustCompile(`(?i)(?:v_show/id_|player\.php/sid/|youku\.com/embed/|video\.tudou\.com/v/)([A-Za-z0-9=]+)`)
)

type Extractor struct{}

func (e *Extractor) Name() string { return "youku" }

func (e *Extractor) Match(rawURL string) bool {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = rawURL
	}
	lu := strings.ToLower(u)
	return strings.Contains(lu, "youku.com") || strings.Contains(lu, "tudou.com")
}

func (e *Extractor) Extract(client *httpx.Client, rawURL string) (*model.VideoInfo, error) {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = strings.TrimSpace(rawURL)
	}
	vid, err := resolveVID(client, u)
	if err != nil {
		return nil, err
	}
	ensureYoukuCookies(client)
	cna, err := fetchCNA(client)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Referer":    u,
		"User-Agent": httpx.DefaultUA,
	}
	var lastErr error
	for _, try := range []struct {
		ccode string
		ckey  string
	}{
		{"0564", youkuCKey},
		{"0564", ""},
		{"0502", luxCKey},
		{"0512", youkuCKey},
	} {
		data, err := fetchUPS(client, vid, cna, try.ccode, try.ckey, headers)
		if err != nil {
			lastErr = err
			continue
		}
		if errObj := util.AsMap(data["error"]); errObj != nil && util.ToInt64(errObj["code"]) != 0 {
			lastErr = fmt.Errorf("优酷错误 %v: %s", errObj["code"], strings.TrimSpace(util.AsString(errObj["note"])))
			continue
		}
		info, err := parseUPS(vid, u, data)
		if err != nil {
			lastErr = err
			continue
		}
		return info, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("优酷 UPS 无可用流")
	}
	return nil, lastErr
}

func resolveVID(client *httpx.Client, rawURL string) (string, error) {
	if m := vidRe.FindStringSubmatch(rawURL); len(m) > 1 {
		return strings.TrimRight(m[1], "."), nil
	}
	body, _, err := client.GetString(rawURL, map[string]string{
		"Referer": "https://v.youku.com/",
	})
	if err != nil {
		return "", fmt.Errorf("无法识别优酷 vid: %s", rawURL)
	}
	if m := regexp.MustCompile(`videoId2\s*[:=]\s*"([A-Za-z0-9=]+)"`).FindStringSubmatch(body); len(m) > 1 {
		return m[1], nil
	}
	if m := vidRe.FindStringSubmatch(body); len(m) > 1 {
		return m[1], nil
	}
	return "", fmt.Errorf("无法识别优酷 vid: %s", rawURL)
}

func ensureYoukuCookies(client *httpx.Client) {
	u, _ := url.Parse("https://v.youku.com/")
	ysuid := fmt.Sprintf("%d%s", time.Now().Unix(), randLetters(3))
	client.HTTP().Jar.SetCookies(u, []*http.Cookie{
		{Name: "__ysuid", Value: ysuid, Domain: ".youku.com", Path: "/"},
		{Name: "xreferrer", Value: "http://www.youku.com", Domain: ".youku.com", Path: "/"},
	})
}

func randLetters(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// fetchCNA 获取 UPS 所需的 utid（cna）。
// mmstat eg.js 偶发 EOF/超时；此处用短超时单次请求，失败即回退默认值（与 you-get 一致），不阻断解析。
func fetchCNA(client *httpx.Client) (string, error) {
	const fallback = "DOG4EdW4qzsCAbZyXbU+t7Jt"
	if cna := cookieValue(client, "https://v.youku.com/", "cna"); cna != "" {
		return cna, nil
	}
	if cna := cookieValue(client, "https://log.mmstat.com/", "cna"); cna != "" {
		return cna, nil
	}

	probe := &http.Client{
		Timeout:   5 * time.Second,
		Jar:       client.HTTP().Jar,
		Transport: client.HTTP().Transport,
	}
	for _, api := range []string{
		"https://log.mmstat.com/eg.js",
		"http://log.mmstat.com/eg.js",
	} {
		req, err := http.NewRequest(http.MethodGet, api, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", httpx.DefaultUA)
		req.Header.Set("Referer", "https://v.youku.com/")
		resp, err := probe.Do(req)
		if err != nil {
			continue
		}
		cna := cnaFromResponse(resp)
		_ = resp.Body.Close()
		if cna == "" {
			cna = cookieValue(client, "https://log.mmstat.com/", "cna")
		}
		if cna != "" {
			setYoukuCNA(client, cna)
			return cna, nil
		}
	}
	setYoukuCNA(client, fallback)
	return fallback, nil
}

func cookieValue(client *httpx.Client, rawURL, name string) string {
	u, err := url.Parse(rawURL)
	if err != nil || client.HTTP().Jar == nil {
		return ""
	}
	for _, c := range client.HTTP().Jar.Cookies(u) {
		if c.Name == name && c.Value != "" {
			return c.Value
		}
	}
	return ""
}

func cnaFromResponse(resp *http.Response) string {
	if etag := strings.Trim(resp.Header.Get("ETag"), `"`); etag != "" {
		return etag
	}
	for _, c := range resp.Cookies() {
		if c.Name == "cna" && c.Value != "" {
			return c.Value
		}
	}
	return ""
}

func setYoukuCNA(client *httpx.Client, cna string) {
	if client.HTTP().Jar == nil || cna == "" {
		return
	}
	u, _ := url.Parse("https://v.youku.com/")
	client.HTTP().Jar.SetCookies(u, []*http.Cookie{
		{Name: "cna", Value: cna, Domain: ".youku.com", Path: "/"},
	})
}

func fetchUPS(client *httpx.Client, vid, cna, ccode, ckey string, headers map[string]string) (map[string]any, error) {
	q := url.Values{}
	q.Set("vid", vid)
	q.Set("ccode", ccode)
	q.Set("client_ip", "192.168.1.1")
	q.Set("utid", cna)
	q.Set("client_ts", fmt.Sprintf("%d", time.Now().Unix()))
	if ckey != "" {
		q.Set("ckey", ckey)
	}
	api := "https://ups.youku.com/ups/get.json?" + q.Encode()
	body, resp, err := client.GetBytes(api, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("UPS HTTP %d", resp.StatusCode)
	}
	var wrap map[string]any
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, fmt.Errorf("UPS JSON 失败: %w", err)
	}
	data := util.AsMap(wrap["data"])
	if data == nil {
		return nil, fmt.Errorf("UPS 无 data")
	}
	return data, nil
}

func parseUPS(vid, pageURL string, data map[string]any) (*model.VideoInfo, error) {
	video := util.AsMap(data["video"])
	if video == nil {
		return nil, fmt.Errorf("UPS 无 video")
	}
	title := util.AsString(video["title"])
	show := util.AsMap(data["show"])
	if showTitle := util.AsString(show["title"]); showTitle != "" && !strings.Contains(title, showTitle) {
		title = showTitle + " " + title
	}
	info := &model.VideoInfo{
		Platform:   "youku",
		ID:         vid,
		Title:      title,
		Author:     util.AsString(video["username"]),
		AuthorID:   fmt.Sprint(util.ToInt64(video["userid"])),
		Duration:   util.ToFloat(video["seconds"]),
		CoverURL:   util.AsString(video["logo"]),
		WebpageURL: pageURL,
	}
	if info.Title == "" {
		info.Title = vid
	}
	if info.AuthorID == "0" {
		info.AuthorID = ""
	}
	headers := map[string]string{"Referer": "https://v.youku.com/"}
	streams, _ := data["stream"].([]any)
	for _, s := range streams {
		sm := util.AsMap(s)
		if sm == nil || util.AsString(sm["channel_type"]) == "tail" {
			continue
		}
		st := util.AsString(sm["stream_type"])
		w := int(util.ToInt64(sm["width"]))
		h := int(util.ToInt64(sm["height"]))
		f := model.Format{
			FormatID:   st,
			Ext:        "mp4",
			Width:      w,
			Height:     h,
			Filesize:   util.ToInt64(sm["size"]),
			HasVideo:   true,
			HasAudio:   true,
			Quality:    fmt.Sprintf("%s %dx%d", st, w, h),
			Preference: h,
			Headers:    headers,
		}
		var parts []string
		if segs, ok := sm["segs"].([]any); ok {
			for _, seg := range segs {
				if u := util.AsString(util.Nested(seg, "cdn_url")); u != "" {
					parts = append(parts, u)
				}
			}
		}
		if len(parts) > 0 {
			f.URL = parts[0]
			if len(parts) > 1 {
				f.PartURLs = parts
			}
		} else if m3u8 := util.AsString(sm["m3u8_url"]); m3u8 != "" {
			f.URL = m3u8
			f.Protocol = "m3u8"
			f.Ext = "mp4"
		} else {
			continue
		}
		info.Formats = append(info.Formats, f)
	}
	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("未找到可下载流（可能地区限制 / 需登录 Cookie）: %s", vid)
	}
	return info, nil
}
