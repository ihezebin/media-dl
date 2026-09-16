package douyin

import (
	"crypto/rc4"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/tjfoc/gmsm/sm3"
)

// Douyin web a_bogus（SM3 + RC4）。uaCode 与 webAPIUA 需配套。
var uaCode = []byte{
	76, 98, 15, 131, 97, 245, 224, 133, 122, 199, 241, 166, 79, 32, 90, 191,
	128, 126, 122, 98, 66, 11, 14, 40, 49, 110, 110, 173, 67, 96, 138, 252,
}

const (
	browserStr = "1536|742|1536|864|0|0|0|0|1536|864|1536|864|1536|742|24|24|MacIntel"
	webAPIUA   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.212 Safari/537.36"
)

var aBogusB64 = base64.NewEncoding("Dkdpgh2ZmsQB80/MfvV36XI1R45-WUAlEixNLwoqYTOPuzKFjJnry79HbGcaStCe").WithPadding(base64.StdPadding)

func sm3Hash(data []byte) []byte {
	return sm3.Sm3Sum(data)
}

func randomList(r float64, b, c, d, e, f, g int) []byte {
	v1 := int(r) & 255
	v2 := int(r) >> 8
	return []byte{
		byte(v1&b | d),
		byte(v1&c | e),
		byte(v2&b | f),
		byte(v2&c | g),
	}
}

func generateString1(rn1, rn2, rn3 float64) string {
	l1 := randomList(rn1, 170, 85, 1, 2, 5, 45&170)
	l2 := randomList(rn2, 170, 85, 1, 0, 0, 0)
	l3 := randomList(rn3, 170, 85, 1, 0, 5, 0)
	return string(l1) + string(l2) + string(l3)
}

func list4(a, b, c, d, e, f, g, h, i, j, k, m, n, o, p, q, r int) []byte {
	return []byte{
		44, byte(a), 0, 0, 0, 0, 24, byte(b), byte(n), 0, byte(c), byte(d), 0, 0, 0, 1, 0, 239,
		byte(e), byte(o), byte(f), byte(g), 0, 0, 0, 0, byte(h), 0, 0, 14, byte(i), byte(j), 0,
		byte(k), byte(m), 3, byte(p), 1, byte(q), 1, byte(r), 0, 0, 0,
	}
}

func generateString2(params, method string, startTime, endTime int64) string {
	paramsArray := sm3Hash(sm3Hash([]byte(params + "cus")))
	methodArray := sm3Hash(sm3Hash([]byte(method + "cus")))

	a := list4(
		int((endTime>>24)&255),
		int(paramsArray[21]),
		int(uaCode[23]),
		int((endTime>>16)&255),
		int(paramsArray[22]),
		int(uaCode[24]),
		int((endTime>>8)&255),
		int((endTime>>0)&255),
		int((startTime>>24)&255),
		int((startTime>>16)&255),
		int((startTime>>8)&255),
		int((startTime>>0)&255),
		int(methodArray[21]),
		int(methodArray[22]),
		int((endTime>>32)&255),
		int((startTime>>32)&255),
		len(browserStr),
	)

	var e byte
	for _, b := range a {
		e ^= b
	}
	a = append(a, []byte(browserStr)...)
	a = append(a, e)

	cipher, err := rc4.NewCipher([]byte("y"))
	if err != nil {
		panic(err)
	}
	dst := make([]byte, len(a))
	cipher.XORKeyStream(dst, a)
	return string(dst)
}

func generateABogus(paramsQuery string) string {
	now := time.Now().UnixMilli()
	endTime := now + int64(rand.Intn(5)+4)
	rn1 := rand.Float64() * 10000
	rn2 := rand.Float64() * 10000
	rn3 := rand.Float64() * 10000
	final := generateString1(rn1, rn2, rn3) + generateString2(paramsQuery, "GET", now, endTime)
	return aBogusB64.EncodeToString([]byte(final))
}

// encodeWebParams 按抖音校验期望的固定键序拼接 query（顺序影响 a_bogus）。
func encodeWebParams(params map[string]string) string {
	ordered := []string{
		"device_platform", "aid", "channel", "pc_client_type",
		"version_code", "version_name", "cookie_enabled",
		"screen_width", "screen_height", "browser_language",
		"browser_platform", "browser_name", "browser_version",
		"browser_online", "engine_name", "engine_version",
		"os_name", "os_version", "cpu_core_num", "device_memory",
		"platform", "downlink", "effective_type", "from_user_page",
		"locate_query", "need_time_list", "pc_libra_divert",
		"publish_video_strategy_type", "round_trip_time",
		"show_live_replay_strategy", "time_list_query",
		"whale_cut_token", "update_version_code", "msToken",
		"max_cursor", "count", "sec_user_id", "aweme_id", "request_source", "origin_type",
	}
	visited := make(map[string]bool, len(params))
	var pairs []string
	for _, k := range ordered {
		if v, ok := params[k]; ok {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, webQueryEscape(v)))
			visited[k] = true
		}
	}
	for k, v := range params {
		if !visited[k] {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, webQueryEscape(v)))
		}
	}
	return strings.Join(pairs, "&")
}

func defaultWebParams(awemeID string) map[string]string {
	return map[string]string{
		"device_platform":     "webapp",
		"aid":                 "6383",
		"channel":             "channel_pc_web",
		"pc_client_type":      "1",
		"version_code":        "190500",
		"version_name":        "19.5.0",
		"cookie_enabled":      "true",
		"screen_width":        "1920",
		"screen_height":       "1080",
		"browser_language":    "zh-CN",
		"browser_platform":    "MacIntel",
		"browser_name":        "Chrome",
		"browser_version":     "120.0.0.0",
		"browser_online":      "true",
		"engine_name":         "Blink",
		"engine_version":      "120.0.0.0",
		"os_name":             "Mac OS",
		"os_version":          "10.15.7",
		"cpu_core_num":        "8",
		"device_memory":       "8",
		"platform":            "PC",
		"downlink":            "10",
		"effective_type":      "4g",
		"round_trip_time":     "50",
		"update_version_code": "190500",
		"aweme_id":            awemeID,
		"msToken":             "",
	}
}
