package iqiyi

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
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

// 参考 lux VPS（分段 mp4）与 yt-dlp / you-get tmts（m3u8）。
// 站点已改为 SPA，tvid 优先从 accelerator.js（Referer=播放页）或 URL slug 的 patchToID 还原。

var (
	tvidRe = []*regexp.Regexp{
		regexp.MustCompile(`(?i)[#?&]tvid=(\d+)`),
		regexp.MustCompile(`data-(?:player|shareplattrigger)-tvid\s*=\s*["'](\d+)`),
		regexp.MustCompile(`param\s*\[\s*'tvid'\s*\]\s*=\s*"(\d+)"`),
		regexp.MustCompile(`"tvId"\s*:\s*"?(\d+)"?`),
		regexp.MustCompile(`"tvid"\s*:\s*"?(\d+)"?`),
	}
	vidRe = []*regexp.Regexp{
		regexp.MustCompile(`(?i)[#?&]vid=([0-9a-f]{32})`),
		regexp.MustCompile(`data-(?:player|shareplattrigger)-videoid\s*=\s*["']([0-9a-f]+)`),
		regexp.MustCompile(`param\s*\[\s*'vid'\s*\]\s*=\s*"([0-9a-f]+)"`),
		regexp.MustCompile(`"vid"\s*:\s*"([0-9a-f]{32})"`),
	}
	titleRe   = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']+)`)
	slugRe    = regexp.MustCompile(`(?i)/[vwp]_([^./?#]+)\.html`)
	prophetRe = regexp.MustCompile(`QiyiPlayerProphetData\s*=`)
)

var vdPref = map[string]int{
	"96": 216, "1": 360, "2": 480, "21": 504, "4": 720, "17": 720, "5": 1080, "18": 1080,
}

type Extractor struct{}

func (e *Extractor) Name() string { return "iqiyi" }

func (e *Extractor) Match(rawURL string) bool {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = rawURL
	}
	lu := strings.ToLower(u)
	return strings.Contains(lu, "iqiyi.com") || strings.Contains(lu, "iq.com") || strings.Contains(lu, "pps.tv")
}

func (e *Extractor) Extract(client *httpx.Client, rawURL string) (*model.VideoInfo, error) {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = strings.TrimSpace(rawURL)
	}
	headers := map[string]string{
		"Referer":         "https://www.iqiyi.com/",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9",
	}
	body, resp, err := client.GetString(u, headers)
	if err != nil {
		return nil, fmt.Errorf("请求爱奇艺页面失败: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("爱奇艺页面 HTTP %d", resp.StatusCode)
	}
	pageURL := u
	if resp.Request != nil {
		pageURL = resp.Request.URL.String()
	}

	tvid := firstMatch(body, tvidRe)
	if tvid == "" {
		tvid = firstMatch(u, tvidRe)
	}
	if tvid == "" {
		tvid = tvidFromQuery(pageURL)
	}
	if tvid == "" {
		tvid = patchToID(pageURL)
	}

	title := ""
	cover := ""
	duration := 0.0
	offline := false
	if prophet, err := fetchAccelerator(client, pageURL); err == nil && prophet != nil {
		if id := util.ToInt64(prophet["tvid"]); id > 0 {
			tvid = strconv.FormatInt(id, 10)
		} else if id := util.ToInt64(prophet["tvId"]); id > 0 {
			tvid = strconv.FormatInt(id, 10)
		}
		vi := util.AsMap(prophet["videoInfo"])
		title = util.FirstString(vi, "title")
		cover = util.FirstString(vi, "imageUrl")
		if util.ToInt64(prophet["offline"]) != 0 || util.AsString(prophet["error"]) == "video offline" {
			offline = true
		}
		if util.FirstString(vi, "pagePublishStatus") == "PAGE_OFFLINE" {
			offline = true
		}
	}

	vid := firstMatch(body, vidRe)
	if vid == "" {
		vid = firstMatch(u, vidRe)
	}

	if tvid != "" {
		if meta, err := fetchBaseinfo(client, tvid); err == nil && meta != nil {
			if vid == "" {
				vid = util.FirstString(meta, "vid", "vu")
			}
			if title == "" {
				title = util.FirstString(meta, "name", "title", "subtitle")
			}
			if cover == "" {
				cover = util.FirstString(meta, "imageUrl", "albumImageUrl")
			}
			if duration == 0 {
				duration = util.ToFloat(meta["duration"])
			}
		}
	}
	if tvid != "" && vid == "" {
		vid = fetchPlayerVID(client, tvid)
	}

	if tvid == "" {
		return nil, fmt.Errorf("无法解析 tvid（SPA 页可尝试完整播放页链接）")
	}
	if offline {
		if title == "" {
			title = tvid
		}
		return nil, fmt.Errorf("视频已下线: %s", title)
	}
	if vid == "" {
		return nil, fmt.Errorf("无法解析 vid（可尝试 --cookies）")
	}

	if title == "" {
		if m := titleRe.FindStringSubmatch(body); len(m) > 1 {
			title = m[1]
		}
	}
	if title == "" {
		if m := regexp.MustCompile(`(?i)<title>([^<]+)</title>`).FindStringSubmatch(body); len(m) > 1 {
			title = strings.TrimSpace(strings.Split(m[1], "-")[0])
		}
	}
	if title == "" {
		title = tvid
	}

	info := &model.VideoInfo{
		Platform:   "iqiyi",
		ID:         vid,
		Title:      title,
		Duration:   duration,
		CoverURL:   cover,
		WebpageURL: pageURL,
	}
	if info.CoverURL == "" {
		if m := regexp.MustCompile(`(?i)<meta[^>]+property=["']og:image["'][^>]+content=["']([^"']+)`).FindStringSubmatch(body); len(m) > 1 {
			info.CoverURL = m[1]
		}
	}

	dlHeaders := map[string]string{"Referer": "https://www.iqiyi.com/"}
	if fmts, err := fetchVPS(client, tvid, vid, dlHeaders); err == nil {
		info.Formats = append(info.Formats, fmts...)
	}
	if len(info.Formats) == 0 {
		if fmts, err := fetchTMTS(client, tvid, vid, dlHeaders); err == nil {
			info.Formats = append(info.Formats, fmts...)
		} else {
			return nil, fmt.Errorf("解析播放地址失败: %w（VIP / 地区限制可尝试 --cookies）", err)
		}
	}
	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("未找到可下载流: %s", vid)
	}
	return info, nil
}

func firstMatch(s string, res []*regexp.Regexp) string {
	for _, re := range res {
		if m := re.FindStringSubmatch(s); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

func fetchAccelerator(client *httpx.Client, pageURL string) (map[string]any, error) {
	const api = "https://mesh.if.iqiyi.com/player/lw/lwplay/accelerator.js?apiVer=3"
	body, _, err := client.GetString(api, map[string]string{
		"Referer": pageURL,
		"Accept":  "*/*",
	})
	if err != nil {
		return nil, err
	}
	idx := prophetRe.FindStringIndex(body)
	if idx == nil {
		return nil, fmt.Errorf("accelerator 无 QiyiPlayerProphetData")
	}
	raw, err := util.ExtractBalancedJSON(body, idx[1])
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func fetchBaseinfo(client *httpx.Client, tvid string) (map[string]any, error) {
	api := "https://pcw-api.iqiyi.com/video/video/baseinfo/" + url.PathEscape(tvid)
	body, _, err := client.GetBytes(api, map[string]string{
		"Referer": "https://www.iqiyi.com/",
		"Accept":  "application/json",
	})
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return util.AsMap(raw["data"]), nil
}

func fetchPlayerVID(client *httpx.Client, tvid string) string {
	api := "https://pcw-api.iqiyi.com/video/video/playervideoinfo?tvid=" + url.QueryEscape(tvid)
	body, _, err := client.GetBytes(api, map[string]string{
		"Referer": "https://www.iqiyi.com/",
		"Accept":  "application/json",
	})
	if err != nil {
		return ""
	}
	var raw map[string]any
	if json.Unmarshal(body, &raw) != nil {
		return ""
	}
	data := util.AsMap(raw["data"])
	return util.FirstString(data, "vid", "vu")
}

func tvidFromQuery(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	q := u.Query()
	for _, k := range []string{"tvid", "tvId"} {
		if v := strings.TrimSpace(q.Get(k)); v != "" {
			return v
		}
	}
	for _, k := range []string{"shareId", "positiveId"} {
		v := strings.TrimSpace(q.Get(k))
		if v == "" {
			continue
		}
		dec, err := url.QueryUnescape(v)
		if err != nil {
			dec = v
		}
		b, err := base64.StdEncoding.DecodeString(dec)
		if err != nil {
			b, err = base64.RawStdEncoding.DecodeString(dec)
		}
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(b))
		if _, err := strconv.ParseInt(s, 10, 64); err == nil {
			return s
		}
	}
	return ""
}

// patchToID 还原播放页 slug（如 v_19rrojlavg.html）对应的 tvid。
func patchToID(rawURL string) string {
	m := slugRe.FindStringSubmatch(rawURL)
	if len(m) < 2 {
		return ""
	}
	val, err := strconv.ParseUint(m[1], 36, 64)
	if err != nil {
		return ""
	}
	const key uint64 = 0x75706971676c
	nBits := reverseBin(key)
	vBits := reverseBin(val)
	xor := make([]byte, len(vBits))
	for i, b := range vBits {
		nb := byte('0')
		if i < len(nBits) {
			nb = nBits[i]
		}
		if b != nb {
			xor[i] = '1'
		} else {
			xor[i] = '0'
		}
	}
	for i, j := 0, len(xor)-1; i < j; i, j = i+1, j-1 {
		xor[i], xor[j] = xor[j], xor[i]
	}
	n, err := strconv.ParseInt(string(xor), 2, 64)
	if err != nil {
		return ""
	}
	if n < 900000 {
		n = 100 * (n + 900000)
	}
	return strconv.FormatInt(n, 10)
}

func reverseBin(n uint64) []byte {
	s := []byte(strconv.FormatUint(n, 2))
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func getMacID() string {
	const chars = "abcdefghijklnmoqprstuvwxyz0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func getVF(params string) string {
	var suffix strings.Builder
	for j := 0; j < 8; j++ {
		for k := 0; k < 4; k++ {
			v4 := 13 * (66*k + 27*j) % 35
			v8 := v4 + 49
			if v4 >= 10 {
				v8 = v4 + 88
			}
			suffix.WriteRune(rune(v8))
		}
	}
	return md5Hex(params + suffix.String())
}

func fetchVPS(client *httpx.Client, tvid, vid string, headers map[string]string) ([]model.Format, error) {
	t := time.Now().Unix() * 1000
	params := fmt.Sprintf(
		"/vps?tvid=%s&vid=%s&v=0&qypid=%s_12&src=01012001010000000000&t=%d&k_tag=1&k_uid=%s&rs=1",
		tvid, vid, tvid, t, getMacID(),
	)
	api := "https://cache.video.iqiyi.com" + params + "&vf=" + getVF(params)
	body, resp, err := client.GetBytes(api, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("VPS HTTP %d", resp.StatusCode)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if util.AsString(raw["code"]) != "A00000" {
		return nil, fmt.Errorf("VPS code=%v msg=%v", raw["code"], raw["msg"])
	}
	vp := util.AsMap(util.Nested(raw, "data", "vp"))
	if vp == nil {
		return nil, fmt.Errorf("VPS 无 vp")
	}
	prefix := util.AsString(vp["du"])
	tkl, _ := vp["tkl"].([]any)
	if len(tkl) == 0 {
		return nil, fmt.Errorf("VPS 无 tkl")
	}
	vs := util.AsSlice(util.Nested(tkl[0], "vs"))
	var out []model.Format
	for _, v := range vs {
		vm := util.AsMap(v)
		fs, _ := vm["fs"].([]any)
		var parts []string
		var size int64
		for _, f := range fs {
			fm := util.AsMap(f)
			l := util.AsString(fm["l"])
			if l == "" {
				continue
			}
			realURL, err := resolveVPSURL(client, prefix+l, headers)
			if err != nil || realURL == "" {
				continue
			}
			parts = append(parts, realURL)
			size += util.ToInt64(fm["b"])
		}
		if len(parts) == 0 {
			continue
		}
		scrsz := util.AsString(vm["scrsz"])
		h := 0
		if i := strings.IndexByte(scrsz, 'x'); i > 0 {
			h, _ = strconv.Atoi(scrsz[i+1:])
		}
		f := model.Format{
			FormatID:   strconv.Itoa(int(util.ToInt64(vm["bid"]))),
			URL:        parts[0],
			Ext:        "mp4",
			Height:     h,
			Filesize:   size,
			HasVideo:   true,
			HasAudio:   true,
			Quality:    scrsz,
			Preference: h,
			Headers:    headers,
		}
		if len(parts) > 1 {
			f.PartURLs = parts
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("VPS 无可用分段")
	}
	return out, nil
}

func resolveVPSURL(client *httpx.Client, api string, headers map[string]string) (string, error) {
	body, _, err := client.GetBytes(api, headers)
	if err != nil {
		return "", err
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", err
	}
	return util.AsString(raw["l"]), nil
}

func fetchTMTS(client *httpx.Client, tvid, vid string, headers map[string]string) ([]model.Format, error) {
	const key = "d5fb4bd9d50c4be6948c97edd7254b0e"
	const src = "76f90cbd92f94a2e925d83e8ccd22cb7"
	var lastErr error
	for i := 0; i < 3; i++ {
		tm := strconv.FormatInt(time.Now().UnixMilli(), 10)
		sc := md5Hex(tm + key + tvid)
		q := url.Values{}
		q.Set("tvid", tvid)
		q.Set("vid", vid)
		q.Set("src", src)
		q.Set("sc", sc)
		q.Set("t", tm)
		api := fmt.Sprintf("http://cache.m.iqiyi.com/jp/tmts/%s/%s/?%s", tvid, vid, q.Encode())
		body, resp, err := client.GetString(api, headers)
		if err != nil {
			lastErr = err
			time.Sleep(time.Second)
			continue
		}
		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("tmts HTTP %d", resp.StatusCode)
			continue
		}
		js := strings.TrimPrefix(strings.TrimSpace(body), "var tvInfoJs=")
		var raw map[string]any
		if err := json.Unmarshal([]byte(js), &raw); err != nil {
			lastErr = err
			continue
		}
		if util.AsString(raw["code"]) != "A00000" {
			lastErr = fmt.Errorf("tmts code=%v", raw["code"])
			if util.AsString(raw["code"]) == "A00111" {
				return nil, fmt.Errorf("地区限制")
			}
			time.Sleep(time.Second)
			continue
		}
		data := util.AsMap(raw["data"])
		vidl, _ := data["vidl"].([]any)
		var out []model.Format
		for _, s := range vidl {
			sm := util.AsMap(s)
			m3u := util.AsString(sm["m3utx"])
			if m3u == "" {
				continue
			}
			vd := fmt.Sprint(sm["vd"])
			out = append(out, model.Format{
				FormatID:   vd,
				URL:        m3u,
				Ext:        "mp4",
				HasVideo:   true,
				HasAudio:   true,
				Quality:    vd,
				Preference: vdPref[vd],
				Protocol:   "m3u8",
				Headers:    headers,
			})
		}
		if len(out) > 0 {
			return out, nil
		}
		lastErr = fmt.Errorf("tmts 无 m3utx")
		time.Sleep(time.Second)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("tmts 失败")
	}
	return nil, lastErr
}
