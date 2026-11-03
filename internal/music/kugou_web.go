package music

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/kugou"
	musicmodel "github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

// KgUser.Cookie.read("KuGoo", "t"/"KugooID") reads fields inside a
// compound Cookie, not standalone cookies. Follow the web player's identity
// source and never mix a compound account ID with another account's token.
func kugouWebCookies(cookie string) map[string]string {
	cookies := map[string]string{}
	req := http.Request{Header: http.Header{"Cookie": []string{cookie}}}
	for _, c := range req.Cookies() {
		cookies[c.Name] = c.Value
	}
	if compound, ok := cookies["KuGoo"]; ok {
		cookies["t"], cookies["KugooID"] = "", ""
		for _, pair := range strings.Split(compound, "&") {
			key, value, found := strings.Cut(pair, "=")
			if !found || (key != "t" && key != "KugooID") {
				continue
			}
			// '+' is a literal cookie character, not form-urlencoded space.
			if decoded, err := url.PathUnescape(value); err == nil {
				cookies[key] = decoded
			}
		}
	}
	return cookies
}

// Match the public web player's songinfo request (infSign / playbyAudio.js).
// Unlike music-lib's songinfo fallback, guests are allowed and album identity
// is preserved. The platform still decides playback and preview permissions.
func kugouWebParams(song *musicmodel.Song, cookie string) url.Values {
	cookies := kugouWebCookies(cookie)
	now := strconv.FormatInt(time.Now().UnixMilli(), 10)
	mid := firstKugouNonEmpty(cookies["kg_mid"], cookies["mid"], now)
	p := url.Values{
		"srcappid": {"2919"}, "clientver": {"20000"}, "clienttime": {now},
		"appid": {"1014"}, "platid": {"4"}, "mid": {mid}, "uuid": {mid},
		"dfid":  {firstKugouNonEmpty(cookies["kg_dfid"], cookies["dfid"], "-")},
		"token": {cookies["t"]}, "userid": {firstKugouNonEmpty(cookies["KugooID"], "0")},
	}
	if encoded := song.Extra["encode_album_audio_id"]; encoded != "" {
		p.Set("encode_album_audio_id", encoded)
	} else {
		p.Set("hash", firstKugouNonEmpty(song.Extra["file_hash"], song.Extra["hash"], song.ID))
		if album := firstKugouNonEmpty(song.AlbumID, song.Extra["album_id"]); album != "" {
			p.Set("album_id", album)
		}
		if id := song.Extra["album_audio_id"]; id != "" {
			p.Set("album_audio_id", id)
		}
	}
	pairs := make([]string, 0, len(p))
	for key := range p {
		pairs = append(pairs, key+"="+p.Get(key))
	}
	sort.Strings(pairs)
	p.Set("signature", utils.MD5(kugou.KugouSignKey+strings.Join(pairs, "")+kugou.KugouSignKey))
	return p
}

func resolveKugouWeb(song *musicmodel.Song, cookie string) (string, error) {
	return resolveKugouWebRequests(song, cookie, func(params url.Values) ([]byte, error) {
		body, err := utils.Get("https://wwwapi.kugou.com/play/songinfo?"+params.Encode(),
			utils.WithHeader("User-Agent", kugou.PCUserAgent),
			utils.WithHeader("Referer", "https://www.kugou.com/"),
			utils.WithHeader("Cookie", cookie))
		if err != nil {
			return nil, fmt.Errorf("酷狗网页解析请求失败")
		}
		return body, nil
	})
}

// The official player resolves hash/album identity first, then plays using the
// returned encoded album-audio ID. The first URL can still be a preview for a
// member; only the second response describes the selected playback resource.
func resolveKugouWebRequests(song *musicmodel.Song, cookie string, fetch func(url.Values) ([]byte, error)) (string, error) {
	deleteExtra(song, "access_reason")
	var playbackError error
	params := kugouWebParams(song, cookie)
	body, err := fetch(params)
	if err != nil {
		return "", err
	}
	var identity struct {
		Status int `json:"status"`
		Data   struct {
			EncodedID string `json:"encode_album_audio_id"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &identity) == nil && identity.Status == 1 && identity.Data.EncodedID != "" && params.Get("encode_album_audio_id") == "" {
		setExtra(song, "encode_album_audio_id", identity.Data.EncodedID)
		playback, playbackErr := fetch(kugouWebParams(song, cookie))
		if playbackErr == nil {
			value, err := applyKugouWebResponse(song, playback)
			if err == nil {
				return value, nil
			}
			playbackError = err
		}
		// A failed rights refresh must not turn an existing preview into a full song.
	}
	value, err := applyKugouWebResponse(song, body)
	if err == nil && playbackError != nil && strings.Contains(playbackError.Error(), "酷狗网页需要安全验证") {
		setExtra(song, "access_reason", playbackError.Error())
	}
	return value, err
}

func applyKugouWebResponse(song *musicmodel.Song, body []byte) (string, error) {
	var response struct {
		Status int `json:"status"`
		Code   int `json:"err_code"`
		Data   struct {
			URL        string `json:"play_url"`
			Backup     string `json:"play_backup_url"`
			EncodedID  string `json:"encode_album_audio_id"`
			Trial      int    `json:"is_free_part"`
			Ext        string `json:"extname"`
			TransParam struct {
				HashOffset struct {
					StartMS int64 `json:"start_ms"`
					EndMS   int64 `json:"end_ms"`
				} `json:"hash_offset"`
			} `json:"trans_param"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &response) != nil {
		return "", fmt.Errorf("酷狗网页解析响应无效，请稍后重试")
	}
	if response.Code == 30020 {
		return "", fmt.Errorf("酷狗网页需要安全验证，请打开酷狗官网完成验证后重试")
	}
	if response.Status != 1 {
		return "", fmt.Errorf("酷狗网页解析失败（%d），请稍后重试", response.Code)
	}
	value := firstKugouNonEmpty(response.Data.URL, response.Data.Backup)
	if value == "" {
		return "", fmt.Errorf("酷狗未返回播放地址，请确认平台登录及歌曲权限后重试")
	}
	if response.Data.Trial == 1 {
		setExtra(song, "is_trial", "1")
		offset := response.Data.TransParam.HashOffset
		if duration := kugouTrialDuration(offset.StartMS, offset.EndMS); duration != "" {
			setExtra(song, "trial_duration", duration)
		}
	} else {
		deleteExtra(song, "is_trial")
		deleteExtra(song, "trial_duration")
	}
	if response.Data.EncodedID != "" {
		setExtra(song, "encode_album_audio_id", response.Data.EncodedID)
	}
	if ext := normalizeAudioExt(response.Data.Ext); ext != "" {
		song.Ext = ext
	}
	return value, nil
}

// Preserve the mobile endpoint's explicit access reason instead of reducing
// every empty URL to "unavailable, errcode=0".
func resolveKugouMobile(song *musicmodel.Song, cookie string) (string, error) {
	p := url.Values{"cmd": {"playInfo"}, "hash": {firstKugouNonEmpty(song.Extra["file_hash"], song.Extra["hash"], song.ID)}}
	body, err := utils.Get("https://m.kugou.com/app/i/getSongInfo.php?"+p.Encode(), utils.WithHeader("User-Agent", kugou.MobileUserAgent), utils.WithHeader("Referer", "https://m.kugou.com/"), utils.WithHeader("Cookie", cookie))
	if err != nil {
		return "", fmt.Errorf("酷狗移动端解析请求失败，请稍后重试")
	}
	var response struct {
		URL   string          `json:"url"`
		Error json.RawMessage `json:"error"`
		Code  int             `json:"errcode"`
	}
	if json.Unmarshal(body, &response) != nil {
		return "", fmt.Errorf("酷狗移动端响应无效，请稍后重试")
	}
	if response.URL != "" {
		return response.URL, nil
	}
	var reason string
	_ = json.Unmarshal(response.Error, &reason)
	if reason != "" {
		return "", fmt.Errorf("酷狗：%s", reason)
	}
	return "", fmt.Errorf("酷狗未返回音源（%d），请稍后重试", response.Code)
}

// The platform declares preview boundaries in milliseconds; never assume all
// previews last 60 seconds or use the full track's timelength for this.
func kugouTrialDuration(start, end int64) string {
	if start < 0 || end <= start {
		return ""
	}
	return strconv.FormatFloat(float64(end-start)/1000, 'f', -1, 64)
}
