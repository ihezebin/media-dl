package bilibili

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hezebin/media-dl/internal/extractor"
	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/model"
	"github.com/hezebin/media-dl/internal/util"
)

func init() {
	extractor.Register(&Extractor{})
}

// 参考 yt-dlp bilibili extractor：
// - Referer + Origin 固定 bilibili.com，缓解 412
// - WBI 签名 (wts / w_rid)
// - playurl 附带 dm_img_* 指纹参数
// - 无 buvid3 时自动注入

var (
	bvRe     = regexp.MustCompile(`(?i)\b(BV[0-9A-Za-z]+)`)
	avRe     = regexp.MustCompile(`(?i)\bav(\d+)`)
	b23Re    = regexp.MustCompile(`(?i)https?://(?:b23\.tv|bili2233\.cn)/[\w-]+/?`)
	pageRe   = regexp.MustCompile(`[?&]p=(\d+)`)
	initRe   = regexp.MustCompile(`window\.__INITIAL_STATE__\s*=`)
	playRe   = regexp.MustCompile(`window\.__playinfo__\s*=`)
	mixinTab = []int{
		46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49,
		33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13, 37, 48, 7, 16, 24, 55, 40,
		61, 26, 17, 0, 1, 60, 51, 30, 4, 22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11,
		36, 20, 34, 44, 52,
	}
)

type Extractor struct{}

func (e *Extractor) Name() string { return "bilibili" }

func (e *Extractor) Match(rawURL string) bool {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = rawURL
	}
	lu := strings.ToLower(u)
	return strings.Contains(lu, "bilibili.com") ||
		strings.Contains(lu, "b23.tv") ||
		strings.Contains(lu, "bili2233.cn")
}

func (e *Extractor) Extract(client *httpx.Client, rawURL string) (*model.VideoInfo, error) {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = strings.TrimSpace(rawURL)
	}
	ensureBuvid3(client)

	finalURL, err := resolveURL(client, u)
	if err != nil {
		return nil, err
	}

	headers := biliHeaders(finalURL)
	bvid, aid, part := parseIDs(finalURL)
	if bvid == "" && aid == 0 {
		return nil, fmt.Errorf("无法解析 BV/av 号: %s", finalURL)
	}

	view, err := fetchView(client, bvid, aid, headers)
	if err != nil {
		// fallback: 页面 __INITIAL_STATE__
		view, err = fetchViewFromPage(client, finalURL, headers)
		if err != nil {
			return nil, err
		}
	}
	if bvid == "" {
		bvid, _ = view["bvid"].(string)
	}
	if aid == 0 {
		aid = toInt64(view["aid"])
	}
	cid := pickCID(view, part)
	if cid == 0 {
		return nil, fmt.Errorf("无法获取 cid")
	}

	title, _ := view["title"].(string)
	desc, _ := view["desc"].(string)
	cover, _ := view["pic"].(string)
	author := ""
	authorID := ""
	if owner, ok := view["owner"].(map[string]any); ok {
		author, _ = owner["name"].(string)
		if mid := toInt64(owner["mid"]); mid != 0 {
			authorID = strconv.FormatInt(mid, 10)
		}
	}
	duration := float64(toInt64(view["duration"]))

	info := &model.VideoInfo{
		Platform:    "bilibili",
		ID:          bvid,
		Title:       title,
		Description: desc,
		Author:      author,
		AuthorID:    authorID,
		Duration:    duration,
		CoverURL:    cover,
		WebpageURL:  fmt.Sprintf("https://www.bilibili.com/video/%s", bvid),
	}
	if part > 1 {
		info.ID = fmt.Sprintf("%s_p%d", bvid, part)
	}

	play, err := downloadPlayInfo(client, bvid, cid, headers, "4048")
	if err != nil {
		return nil, err
	}
	info.Formats = extractFormats(play, headers)
	// 额外尝试一体流（免 ffmpeg），参考 yt-dlp 对 legacy format 的补充
	if play2, err2 := downloadPlayInfo(client, bvid, cid, headers, "1"); err2 == nil {
		info.Formats = append(info.Formats, extractFormats(play2, headers)...)
	}
	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("未找到可下载清晰度 (可能触发 412/风控，请加 --cookies 或稍后重试)")
	}
	return info, nil
}

func biliHeaders(pageURL string) map[string]string {
	referer := "https://www.bilibili.com/"
	if pageURL != "" {
		referer = pageURL
	}
	return map[string]string{
		"Referer":         referer,
		"Origin":          "https://www.bilibili.com",
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Sec-Fetch-Dest":  "empty",
		"Sec-Fetch-Mode":  "cors",
		"Sec-Fetch-Site":  "same-site",
	}
}

func resolveURL(client *httpx.Client, rawURL string) (string, error) {
	if b23Re.MatchString(rawURL) {
		final, err := client.ResolveRedirect(rawURL, map[string]string{
			"Referer": "https://www.bilibili.com/",
		})
		if err != nil {
			return "", fmt.Errorf("解析 b23 短链失败: %w", err)
		}
		return final, nil
	}
	return rawURL, nil
}

func parseIDs(rawURL string) (bvid string, aid int64, part int) {
	part = 1
	if m := pageRe.FindStringSubmatch(rawURL); len(m) > 1 {
		if p, err := strconv.Atoi(m[1]); err == nil && p > 0 {
			part = p
		}
	}
	if m := bvRe.FindStringSubmatch(rawURL); len(m) > 1 {
		bvid = m[1]
		if !strings.HasPrefix(strings.ToUpper(bvid), "BV") {
			bvid = "BV" + bvid
		}
		// 规范化大小写：BV + 原后缀
		if len(bvid) >= 2 {
			bvid = "BV" + bvid[2:]
		}
	}
	if m := avRe.FindStringSubmatch(rawURL); len(m) > 1 {
		aid, _ = strconv.ParseInt(m[1], 10, 64)
	}
	return
}

func ensureBuvid3(client *httpx.Client) {
	u, _ := url.Parse("https://api.bilibili.com/")
	for _, c := range client.HTTP().Jar.Cookies(u) {
		if c.Name == "buvid3" && c.Value != "" {
			return
		}
	}
	client.HTTP().Jar.SetCookies(u, []*http.Cookie{{
		Name:   "buvid3",
		Value:  fmt.Sprintf("%sinfoc", randomHex(8)+"-"+randomHex(4)+"-"+randomHex(4)+"-"+randomHex(4)+"-"+randomHex(12)),
		Domain: ".bilibili.com",
		Path:   "/",
	}})
}

func randomHex(n int) string {
	const hexdigits = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hexdigits[rand.Intn(len(hexdigits))]
	}
	return string(b)
}

var (
	wbiMu  sync.Mutex
	wbiKey string
	wbiTS  time.Time
)

func getWBIKey(client *httpx.Client, headers map[string]string) (string, error) {
	wbiMu.Lock()
	defer wbiMu.Unlock()
	if wbiKey != "" && time.Since(wbiTS) < 30*time.Second {
		return wbiKey, nil
	}
	body, resp, err := client.GetBytes("https://api.bilibili.com/x/web-interface/nav", headers)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == 412 {
		return "", fmt.Errorf("nav 接口 412 (风控)，请提供浏览器 Cookie (--cookies) 或更换网络")
	}
	var nav struct {
		Data struct {
			WbiImg struct {
				ImgURL string `json:"img_url"`
				SubURL string `json:"sub_url"`
			} `json:"wbi_img"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &nav); err != nil {
		return "", err
	}
	img := fileStem(nav.Data.WbiImg.ImgURL)
	sub := fileStem(nav.Data.WbiImg.SubURL)
	lookup := img + sub
	var mixed strings.Builder
	for _, i := range mixinTab {
		if i < len(lookup) {
			mixed.WriteByte(lookup[i])
		}
	}
	key := mixed.String()
	if len(key) > 32 {
		key = key[:32]
	}
	wbiKey = key
	wbiTS = time.Now()
	return wbiKey, nil
}

func fileStem(u string) string {
	base := u
	if i := strings.LastIndex(u, "/"); i >= 0 {
		base = u[i+1:]
	}
	if i := strings.Index(base, "."); i >= 0 {
		base = base[:i]
	}
	return base
}

func signWBI(client *httpx.Client, params map[string]string, headers map[string]string) (url.Values, error) {
	key, err := getWBIKey(client, headers)
	if err != nil {
		return nil, err
	}
	params["wts"] = strconv.FormatInt(time.Now().Unix(), 10)
	filtered := map[string]string{}
	for k, v := range params {
		filtered[k] = filterWBIValue(v)
	}
	keys := make([]string, 0, len(filtered))
	for k := range filtered {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	q := url.Values{}
	for _, k := range keys {
		q.Set(k, filtered[k])
	}
	// 与 yt-dlp/urllib.parse.urlencode 对齐：先 Encode 再算 MD5，再附加 w_rid
	query := q.Encode()
	sum := md5.Sum([]byte(query + key))
	q.Set("w_rid", hex.EncodeToString(sum[:]))
	return q, nil
}

func filterWBIValue(v string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune("!'()*", r) {
			return -1
		}
		return r
	}, v)
}

func dmParams() map[string]string {
	// 参考 yt-dlp _dm_params，降低 playurl 412
	wh := dmWH(1920, 1080)
	of := dmOF(rand.Intn(100), 0)
	inter, _ := json.Marshal(map[string]any{
		"ds": []any{},
		"wh": wh,
		"of": of,
	})
	return map[string]string{
		"dm_img_list":      "[]",
		"dm_img_str":       randB64(16, 64),
		"dm_cover_img_str": randB64(32, 128),
		"dm_img_inter":     string(inter),
	}
}

func dmWH(w, h int) []int {
	rnd := int(math.Floor(114 * rand.Float64()))
	return []int{2*w + 2*h + 3*rnd, 4*w - h + rnd, rnd}
}

func dmOF(top, left int) []int {
	rnd := int(math.Floor(514 * rand.Float64()))
	return []int{3*top + 2*left + rnd, 4*top - 4*left + 2*rnd, rnd}
}

func randB64(minN, maxN int) string {
	n := minN
	if maxN > minN {
		n = minN + rand.Intn(maxN-minN+1)
	}
	const printable = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~ "
	b := make([]byte, n)
	for i := range b {
		b[i] = printable[rand.Intn(len(printable))]
	}
	s := base64.StdEncoding.EncodeToString(b)
	if len(s) >= 2 {
		s = s[:len(s)-2]
	}
	return s
}

func fetchView(client *httpx.Client, bvid string, aid int64, headers map[string]string) (map[string]any, error) {
	params := map[string]string{}
	if bvid != "" {
		params["bvid"] = bvid
	} else {
		params["aid"] = strconv.FormatInt(aid, 10)
	}
	q, err := signWBI(client, params, headers)
	if err != nil {
		return nil, err
	}
	api := "https://api.bilibili.com/x/web-interface/wbi/view?" + q.Encode()
	body, resp, err := client.GetBytes(api, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 412 {
		return nil, fmt.Errorf("view 接口 412，请使用 --cookies 导入浏览器 Cookie")
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	code := toInt64(raw["code"])
	if code != 0 {
		return nil, fmt.Errorf("view API code=%d msg=%v", code, raw["message"])
	}
	data, _ := raw["data"].(map[string]any)
	if data == nil {
		return nil, fmt.Errorf("view API 无 data")
	}
	return data, nil
}

func fetchViewFromPage(client *httpx.Client, pageURL string, headers map[string]string) (map[string]any, error) {
	body, resp, err := client.GetString(pageURL, map[string]string{
		"Referer": "https://www.bilibili.com/",
		"Accept":  "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 412 {
		return nil, fmt.Errorf("页面 412 Precondition Failed：B 站风控。解决办法：1) --cookies 导入登录 Cookie 2) 使用国内网络/代理 3) 稍后重试")
	}
	idx := initRe.FindStringIndex(body)
	if idx == nil {
		return nil, fmt.Errorf("页面无 __INITIAL_STATE__")
	}
	raw, err := util.ExtractBalancedJSON(body, idx[1])
	if err != nil {
		return nil, err
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	if vd, ok := state["videoData"].(map[string]any); ok {
		return vd, nil
	}
	if vi, ok := state["videoInfo"].(map[string]any); ok {
		return vi, nil
	}
	_ = headers
	return nil, fmt.Errorf("无法从页面提取 videoData")
}

func pickCID(view map[string]any, part int) int64 {
	if pages, ok := view["pages"].([]any); ok && len(pages) > 0 {
		idx := part - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(pages) {
			idx = 0
		}
		if p, ok := pages[idx].(map[string]any); ok {
			return toInt64(p["cid"])
		}
	}
	return toInt64(view["cid"])
}

func downloadPlayInfo(client *httpx.Client, bvid string, cid int64, headers map[string]string, fnval string) (map[string]any, error) {
	if fnval == "" {
		fnval = "4048"
	}
	params := map[string]string{
		"bvid":     bvid,
		"cid":      strconv.FormatInt(cid, 10),
		"fnval":    fnval,
		"fourk":    "1",
		"try_look": "1",
	}
	for k, v := range dmParams() {
		params[k] = v
	}
	q, err := signWBI(client, params, headers)
	if err != nil {
		return nil, err
	}
	api := "https://api.bilibili.com/x/player/wbi/playurl?" + q.Encode()
	h := map[string]string{}
	for k, v := range headers {
		h[k] = v
	}
	// playurl 对 Origin/Referer 敏感（yt-dlp #14830 / #16571）
	h["Referer"] = "https://www.bilibili.com/"
	h["Origin"] = "https://www.bilibili.com"

	body, resp, err := client.GetBytes(api, h)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 412 {
		return nil, fmt.Errorf("playurl 412：请加 --cookies，并确认 Referer/Origin；海外 IP 更容易触发")
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	code := toInt64(raw["code"])
	if code != 0 {
		return nil, fmt.Errorf("playurl code=%d msg=%v", code, raw["message"])
	}
	data, _ := raw["data"].(map[string]any)
	if data == nil {
		return nil, fmt.Errorf("playurl 无 data")
	}
	return data, nil
}

func extractFormats(play map[string]any, headers map[string]string) []model.Format {
	var formats []model.Format
	dlHeaders := map[string]string{
		"Referer": "https://www.bilibili.com/",
		"Origin":  "https://www.bilibili.com",
	}
	for k, v := range headers {
		if k == "Referer" || k == "Origin" {
			dlHeaders[k] = v
		}
	}

	// DASH
	if dash, ok := play["dash"].(map[string]any); ok {
		var audioURL string
		var audioID string
		bestAudioBW := int64(-1)
		collectAudio := func(list []any) {
			for _, a := range list {
				am, ok := a.(map[string]any)
				if !ok {
					continue
				}
				bw := toInt64(am["bandwidth"])
				u := firstString(am, "baseUrl", "base_url", "url")
				if u == "" {
					continue
				}
				if bw > bestAudioBW {
					bestAudioBW = bw
					audioURL = u
					audioID = fmt.Sprintf("%v", am["id"])
				}
			}
		}
		if audios, ok := dash["audio"].([]any); ok {
			collectAudio(audios)
		}
		if dolby, ok := dash["dolby"].(map[string]any); ok {
			if audios, ok := dolby["audio"].([]any); ok {
				collectAudio(audios)
			}
		}
		if flac, ok := dash["flac"].(map[string]any); ok {
			if a, ok := flac["audio"].(map[string]any); ok {
				collectAudio([]any{a})
			}
		}

		if videos, ok := dash["video"].([]any); ok {
			for _, v := range videos {
				vm, ok := v.(map[string]any)
				if !ok {
					continue
				}
				u := firstString(vm, "baseUrl", "base_url", "url")
				if u == "" {
					continue
				}
				w := int(toInt64(vm["width"]))
				h := int(toInt64(vm["height"]))
				id := fmt.Sprintf("%v", vm["id"])
				formats = append(formats, model.Format{
					FormatID:   "dash_" + id,
					URL:        u,
					Ext:        "mp4",
					Width:      w,
					Height:     h,
					Filesize:   toInt64(vm["size"]),
					VCodec:     fmt.Sprintf("%v", vm["codecs"]),
					ACodec:     "none",
					HasVideo:   true,
					HasAudio:   audioURL != "",
					AudioURL:   audioURL,
					Quality:    fmt.Sprintf("%dp", h),
					Preference: h,
					Headers:    dlHeaders,
				})
				_ = audioID
			}
		}
	}

	// 传统 durl（音视频一体，优先）
	if durl, ok := play["durl"].([]any); ok && len(durl) > 0 {
		if d0, ok := durl[0].(map[string]any); ok {
			u, _ := d0["url"].(string)
			if u != "" {
				q := int(toInt64(play["quality"]))
				formats = append(formats, model.Format{
					FormatID:   fmt.Sprintf("durl_%d", q),
					URL:        u,
					Ext:        "mp4",
					Filesize:   toInt64(d0["size"]),
					HasVideo:   true,
					HasAudio:   true,
					Quality:    strconv.Itoa(q),
					Preference: 100000 + q, // 一体流优先
					Headers:    dlHeaders,
				})
			}
		}
	}
	return formats
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case json.Number:
		i, _ := t.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(t, 10, 64)
		return i
	default:
		return 0
	}
}
