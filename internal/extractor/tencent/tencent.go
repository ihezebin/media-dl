package tencent

import (
	"crypto/aes"
	"crypto/cipher"
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

// 腾讯视频：优先 you-get / lux 的 getinfo+getkey 分段 MP4；
// 失败时回退 yt-dlp 的 cKey + HLS getvinfo。

const (
	appVer   = "3.2.19.333"
	ytAppVer = "3.5.57"
	ytPlat   = "10901"
)

var (
	vidURLRe = regexp.MustCompile(`(?i)/([a-z0-9]{11})\.html`)
	vidQSRe  = regexp.MustCompile(`(?i)[?&]vid=([a-z0-9]{11})`)
	vidJSRe  = []*regexp.Regexp{
		regexp.MustCompile(`(?i)"vid"\s*:\s*"([a-z0-9]{11})"`),
		regexp.MustCompile(`(?i)vid\s*[:=]\s*["']([a-z0-9]{11})`),
	}
	coverRe = regexp.MustCompile(`(?i)/x/cover/([a-z0-9]+)`)
)

var defnPref = map[string]int{
	"fhd": 1080, "shd": 720, "hd": 480, "sd": 360, "ld": 240,
}

type Extractor struct{}

func (e *Extractor) Name() string { return "tencent" }

func (e *Extractor) Match(rawURL string) bool {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = rawURL
	}
	lu := strings.ToLower(u)
	return strings.Contains(lu, "v.qq.com") ||
		strings.Contains(lu, "film.qq.com") ||
		strings.Contains(lu, "video.qq.com")
}

func (e *Extractor) Extract(client *httpx.Client, rawURL string) (*model.VideoInfo, error) {
	u := util.ExtractFirstURL(rawURL)
	if u == "" {
		u = strings.TrimSpace(rawURL)
	}
	headers := map[string]string{
		"Referer":         u,
		"User-Agent":      httpx.DefaultUA,
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9",
	}
	page, _, _ := client.GetString(u, headers)
	vid := parseVID(u, page)
	if vid == "" {
		return nil, fmt.Errorf("无法解析腾讯视频 vid: %s", u)
	}
	cid := ""
	if m := coverRe.FindStringSubmatch(u); len(m) > 1 {
		cid = m[1]
	}

	info, err := extractByGetinfo(client, vid, u)
	if err == nil {
		fillMeta(info, page, u)
		return info, nil
	}
	last := err
	info, err = extractByCKeyHLS(client, vid, cid, u)
	if err == nil {
		fillMeta(info, page, u)
		return info, nil
	}
	return nil, fmt.Errorf("腾讯视频解析失败: %v; %w（VIP 内容需 --cookies）", last, err)
}

func parseVID(pageURL, html string) string {
	if m := vidQSRe.FindStringSubmatch(pageURL); len(m) > 1 {
		return m[1]
	}
	if m := vidURLRe.FindStringSubmatch(pageURL); len(m) > 1 {
		return m[1]
	}
	for _, re := range vidJSRe {
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

func fillMeta(info *model.VideoInfo, html, pageURL string) {
	info.WebpageURL = pageURL
	if info.Title == "" {
		if m := regexp.MustCompile(`(?i)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']+)`).FindStringSubmatch(html); len(m) > 1 {
			info.Title = cleanTitle(m[1])
		}
	} else {
		info.Title = cleanTitle(info.Title)
	}
	if info.Description == "" {
		if m := regexp.MustCompile(`(?i)<meta[^>]+(?:property=["']og:description["']|name=["']description["'])[^>]+content=["']([^"']+)`).FindStringSubmatch(html); len(m) > 1 {
			info.Description = m[1]
		}
	}
	if info.CoverURL == "" {
		if m := regexp.MustCompile(`(?i)<meta[^>]+property=["']og:image["'][^>]+content=["']([^"']+)`).FindStringSubmatch(html); len(m) > 1 {
			info.CoverURL = m[1]
		}
	}
}

func cleanTitle(t string) string {
	t = regexp.MustCompile(`\s*[_\-]\s*(?:Watch online|WeTV|腾讯视频|(?:高清)?1080P在线观看平台).*$`).ReplaceAllString(t, "")
	return strings.TrimSpace(t)
}

func extractByGetinfo(client *httpx.Client, vid, pageURL string) (*model.VideoInfo, error) {
	headers := map[string]string{
		"Referer":    pageURL,
		"User-Agent": "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) QQLive/10275340/50192209 Chrome/43.0.2357.134 Safari/537.36 QBCore/3.43.561.202 QQBrowser/9.0.2524.400",
	}
	var data map[string]any
	var last error
	for _, platform := range []string{"11", "4100201"} {
		api := fmt.Sprintf("http://vv.video.qq.com/getinfo?otype=json&appver=%s&platform=%s&defnpayver=1&defn=shd&vid=%s",
			appVer, platform, vid)
		body, _, err := client.GetString(api, headers)
		if err != nil {
			last = err
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(util.StripJSONP(strings.TrimPrefix(strings.TrimSpace(body), "QZOutputJson="))), &raw); err != nil {
			last = err
			continue
		}
		if msg := util.AsString(raw["msg"]); msg != "" && msg != "ok" {
			last = fmt.Errorf("%s", msg)
			if msg == "cannot play outside" {
				continue
			}
			// 仍尝试解析 vl
		}
		if vl := util.AsMap(raw["vl"]); vl != nil {
			if vi, _ := vl["vi"].([]any); len(vi) > 0 {
				data = raw
				break
			}
		}
		last = fmt.Errorf("getinfo 无 vl")
	}
	if data == nil {
		if last == nil {
			last = fmt.Errorf("getinfo 失败")
		}
		return nil, last
	}

	viList := util.AsSlice(util.Nested(data, "vl", "vi"))
	if len(viList) == 0 {
		return nil, fmt.Errorf("getinfo 无视频条目")
	}
	vi := util.AsMap(viList[0])
	if vi == nil {
		return nil, fmt.Errorf("getinfo 视频条目为空")
	}
	title := util.AsString(vi["ti"])
	info := &model.VideoInfo{
		Platform: "tencent",
		ID:       vid,
		Title:    title,
	}
	cdn := firstCDN(vi)
	if cdn == "" {
		return nil, fmt.Errorf("getinfo 无 CDN")
	}

	dlHeaders := map[string]string{"Referer": "https://v.qq.com/"}
	fiList := util.AsSlice(util.Nested(data, "fl", "fi"))
	if len(fiList) == 0 {
		// 单流：fn + fvkey
		fn := util.AsString(vi["fn"])
		vkey := util.AsString(vi["fvkey"])
		if fn == "" || vkey == "" {
			return nil, fmt.Errorf("getinfo 无播放文件")
		}
		info.Formats = []model.Format{{
			FormatID:   "default",
			URL:        cdn + fn + "?vkey=" + vkey,
			Ext:        "mp4",
			HasVideo:   true,
			HasAudio:   true,
			Preference: 500,
			Headers:    dlHeaders,
		}}
		return info, nil
	}

	for _, fi := range fiList {
		fm := util.AsMap(fi)
		name := util.AsString(fm["name"])
		formatID := int(util.ToInt64(fm["id"]))
		parts, err := buildParts(client, vid, cdn, vi, name, formatID, headers)
		if err != nil || len(parts) == 0 {
			continue
		}
		f := model.Format{
			FormatID:   name,
			URL:        parts[0],
			Ext:        "mp4",
			Filesize:   util.ToInt64(fm["fs"]),
			HasVideo:   true,
			HasAudio:   true,
			Quality:    util.FirstString(fm, "cname", "name"),
			Preference: defnPref[name],
			Headers:    dlHeaders,
		}
		if len(parts) > 1 {
			f.PartURLs = parts
		}
		info.Formats = append(info.Formats, f)
	}
	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("getkey 未拿到分段")
	}
	return info, nil
}

func buildParts(client *httpx.Client, vid, cdn string, vi map[string]any, defn string, formatID int, headers map[string]string) ([]string, error) {
	fn := util.AsString(vi["fn"])
	fc := int(util.ToInt64(util.Nested(vi, "cl", "fc")))
	fns := strings.Split(fn, ".")
	if defn == "shd" || defn == "fhd" {
		if len(fns) >= 1 {
			idName := fmt.Sprintf("p%d", formatID%10000)
			fns = []string{fns[0], idName, "mp4"}
		}
	} else {
		tmp, err := getinfoDefn(client, vid, defn, headers)
		if err == nil {
			fn = util.AsString(tmp["fn"])
			fns = strings.Split(fn, ".")
			fc = int(util.ToInt64(util.Nested(tmp, "cl", "fc")))
			cdn = firstCDN(tmp)
		}
	}
	if fc == 0 {
		fc = 1
	}
	var parts []string
	for part := 1; part <= fc; part++ {
		cur := append([]string{}, fns...)
		if fc > 1 {
			if len(cur) < 4 {
				// n0687peq62x.p709.mp4 -> n0687peq62x.p709.1.mp4
				if len(cur) >= 2 {
					cur = append(cur[:2], append([]string{strconv.Itoa(part)}, cur[2:]...)...)
				}
			} else {
				cur[2] = strconv.Itoa(part)
			}
		}
		filename := strings.Join(cur, ".")
		keyAPI := fmt.Sprintf("http://vv.video.qq.com/getkey?otype=json&platform=11&appver=%s&filename=%s&format=%d&vid=%s",
			appVer, url.QueryEscape(filename), formatID, vid)
		body, _, err := client.GetString(keyAPI, headers)
		if err != nil {
			continue
		}
		var keyRaw map[string]any
		if err := json.Unmarshal([]byte(util.StripJSONP(strings.TrimPrefix(strings.TrimSpace(body), "QZOutputJson="))), &keyRaw); err != nil {
			continue
		}
		vkey := util.AsString(keyRaw["key"])
		if vkey == "" {
			vkey = util.AsString(vi["fvkey"])
		}
		if vkey == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s%s?vkey=%s", cdn, filename, vkey))
	}
	return parts, nil
}

func getinfoDefn(client *httpx.Client, vid, defn string, headers map[string]string) (map[string]any, error) {
	api := fmt.Sprintf("http://vv.video.qq.com/getinfo?otype=json&platform=11&defnpayver=1&appver=%s&defn=%s&vid=%s",
		appVer, defn, vid)
	body, _, err := client.GetString(api, headers)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(util.StripJSONP(strings.TrimPrefix(strings.TrimSpace(body), "QZOutputJson="))), &raw); err != nil {
		return nil, err
	}
	vi := util.AsSlice(util.Nested(raw, "vl", "vi"))
	if len(vi) == 0 {
		return nil, fmt.Errorf("无 vi")
	}
	return util.AsMap(vi[0]), nil
}

func firstCDN(vi map[string]any) string {
	ui := util.AsSlice(util.Nested(vi, "ul", "ui"))
	if len(ui) == 0 {
		return ""
	}
	return util.AsString(util.Nested(ui[0], "url"))
}

func extractByCKeyHLS(client *httpx.Client, vid, cid, pageURL string) (*model.VideoInfo, error) {
	headers := map[string]string{
		"Referer":    "https://v.qq.com/",
		"User-Agent": httpx.DefaultUA,
	}
	info := &model.VideoInfo{Platform: "tencent", ID: vid, Title: vid}
	seen := map[string]bool{}
	for _, q := range []string{"hd", "shd", "fhd", "sd"} {
		raw, err := getvinfoCKey(client, vid, cid, pageURL, q, headers)
		if err != nil {
			continue
		}
		if msg := util.AsString(raw["msg"]); msg != "" && util.AsString(raw["code"]) != "0.0" && util.AsString(raw["code"]) != "0" {
			continue
		}
		viList := util.AsSlice(util.Nested(raw, "vl", "vi"))
		if len(viList) == 0 {
			continue
		}
		vi := util.AsMap(viList[0])
		if info.Title == vid {
			if t := util.AsString(vi["ti"]); t != "" {
				info.Title = t
			}
		}
		w := int(util.ToInt64(vi["vw"]))
		h := int(util.ToInt64(vi["vh"]))
		ui := util.AsSlice(util.Nested(vi, "ul", "ui"))
		for _, u := range ui {
			um := util.AsMap(u)
			play := util.AsString(um["url"])
			if hls := util.AsMap(um["hls"]); hls != nil {
				play += util.AsString(hls["pt"])
			}
			if play == "" {
				fn := util.AsString(vi["fn"])
				vkey := util.AsString(vi["fvkey"])
				if fn != "" && vkey != "" {
					play = util.AsString(um["url"]) + fn + "?vkey=" + vkey
				}
			}
			if play == "" || seen[play] {
				continue
			}
			seen[play] = true
			f := model.Format{
				FormatID:   q,
				URL:        play,
				Ext:        "mp4",
				Width:      w,
				Height:     h,
				HasVideo:   true,
				HasAudio:   true,
				Quality:    q,
				Preference: defnPref[q] + h,
				Headers:    headers,
			}
			if strings.Contains(strings.ToLower(play), ".m3u8") || util.AsMap(um["hls"]) != nil {
				f.Protocol = "m3u8"
			}
			info.Formats = append(info.Formats, f)
		}
	}
	if len(info.Formats) == 0 {
		return nil, fmt.Errorf("getvinfo/cKey 无可用流")
	}
	return info, nil
}

func getvinfoCKey(client *httpx.Client, vid, cid, pageURL, defn string, headers map[string]string) (map[string]any, error) {
	guid := randAlnum(16)
	ckey, err := makeCKey(vid, pageURL, guid)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("vid", vid)
	q.Set("cid", cid)
	q.Set("cKey", ckey)
	q.Set("encryptVer", "8.1")
	q.Set("spcaptiontype", "0")
	q.Set("sphls", "2")
	q.Set("dtype", "3")
	q.Set("defn", defn)
	q.Set("spsrt", "2")
	q.Set("sphttps", "1")
	q.Set("otype", "json")
	q.Set("spwm", "1")
	q.Set("hevclv", "28")
	q.Set("drm", "40")
	q.Set("spvideo", "4")
	q.Set("spsfrhdr", "100")
	q.Set("host", "v.qq.com")
	q.Set("referer", "v.qq.com")
	q.Set("ehost", pageURL)
	q.Set("appVer", ytAppVer)
	q.Set("platform", ytPlat)
	q.Set("guid", guid)
	q.Set("flowid", randAlnum(32))
	api := "https://h5vv6.video.qq.com/getvinfo?" + q.Encode()
	body, resp, err := client.GetString(api, headers)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("getvinfo HTTP %d", resp.StatusCode)
	}
	var raw map[string]any
	js := strings.TrimPrefix(strings.TrimSpace(body), "QZOutputJson=")
	if err := json.Unmarshal([]byte(util.StripJSONP(js)), &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func randAlnum(n int) string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func makeCKey(videoID, pageURL, guid string) (string, error) {
	ua := strings.ToLower(httpx.DefaultUA)
	if len(pageURL) > 48 {
		pageURL = pageURL[:48]
	}
	if len(ua) > 48 {
		ua = ua[:48]
	}
	payload := fmt.Sprintf("%s|%d|mg3c3b04ba|%s|%s|%s|%s|%s||Mozilla|Netscape|Windows x86_64|00|",
		videoID, time.Now().Unix(), ytAppVer, guid, ytPlat, pageURL, ua)
	sum := 0
	for _, r := range payload {
		sum += int(r)
	}
	plain := []byte(fmt.Sprintf("|%d|%s", sum, payload))
	key := []byte{0x4f, 0x6b, 0xda, 0xa3, 0x9e, 0x2f, 0x8c, 0xb0, 0x7f, 0x5e, 0x72, 0x2d, 0x9e, 0xde, 0xf3, 0x14}
	iv := []byte{0x01, 0x50, 0x4a, 0xf3, 0x56, 0xe6, 0x19, 0xcf, 0x2e, 0x42, 0xbb, 0xa6, 0x8c, 0x3f, 0x70, 0xf9}
	enc, err := aesCBCWhitespace(plain, key, iv)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(enc)), nil
}

func aesCBCWhitespace(plain, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	padded := append(plain, bytesRepeat(byte(' '), pad)...)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return out, nil
}

func bytesRepeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
