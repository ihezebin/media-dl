package music

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/kugou"
	musicmodel "github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

// music-lib intentionally asks Kugou for ten rows. SearchCandidates uses the
// same endpoint with a larger page size so invalid first-page rows can be
// replaced without changing the provider library.
func searchKugouSongs(keyword string, limit int, cookie string) ([]musicmodel.Song, error) {
	if limit <= 0 {
		limit = 10
	}
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("platform", "WebFilter")
	params.Set("format", "json")
	params.Set("page", "1")
	params.Set("pagesize", strconv.Itoa(limit))
	params.Set("userid", "-1")
	params.Set("clientver", "")
	params.Set("tag", "em")
	params.Set("filter", "2")
	params.Set("iscorrection", "1")
	params.Set("privilege_filter", "0")
	params.Set("_", strconv.FormatInt(time.Now().UnixMilli(), 10))
	apiURL := "http://songsearch.kugou.com/song_search_v2?" + params.Encode()
	fetch := func(withCookie bool) ([]byte, error) {
		opts := []utils.RequestOption{
			utils.WithHeader("User-Agent", "Mozilla/5.0 (Linux; Android 10; SM-G981B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.162 Mobile Safari/537.36"),
			utils.WithRandomIPHeader(),
		}
		if withCookie && strings.TrimSpace(cookie) != "" {
			opts = append(opts, utils.WithHeader("Cookie", cookie))
		}
		return utils.Get(apiURL, opts...)
	}

	body, err := fetch(true)
	if err != nil {
		return nil, err
	}
	var response kugouSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("酷狗搜索响应解析失败: %w", err)
	}
	if len(response.Data.Lists) == 0 && strings.TrimSpace(cookie) != "" {
		if retryBody, retryErr := fetch(false); retryErr == nil {
			var retryResponse kugouSearchResponse
			if json.Unmarshal(retryBody, &retryResponse) == nil && len(retryResponse.Data.Lists) > 0 {
				response = retryResponse
			}
		}
	}

	isVIP, _ := kugou.New(cookie).IsVipAccount()
	songs := make([]musicmodel.Song, 0, len(response.Data.Lists))
	for _, item := range response.Data.Lists {
		finalHash := item.FileHash
		if item.Privilege != 10 && isVIP {
			finalHash = item.SQFileHash
		}
		if strings.TrimSpace(finalHash) == "" {
			finalHash = firstKugouNonEmpty(item.SQFileHash, item.HQFileHash, item.ResFileHash, item.TransParam.Ogg320Hash, item.FileHash, item.TransParam.Ogg128Hash)
		}
		if strings.TrimSpace(finalHash) == "" {
			continue
		}

		size := kugouItemSize(item, finalHash)
		bitrate := 0
		if item.Duration > 0 && size > 0 {
			bitrate = int(size * 8 / 1000 / int64(item.Duration))
		}
		cover := strings.Replace(item.Image, "{size}", "240", 1)
		songs = append(songs, musicmodel.Song{
			Source:   "kugou",
			ID:       finalHash,
			Name:     cleanKugouText(item.SongName),
			Artist:   cleanKugouText(item.SingerName),
			Album:    cleanKugouText(item.AlbumName),
			AlbumID:  item.AlbumID,
			Duration: item.Duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", finalHash),
			Extra: map[string]string{
				"hash":           finalHash,
				"ogg_320_hash":   item.TransParam.Ogg320Hash,
				"ogg_128_hash":   item.TransParam.Ogg128Hash,
				"sq_hash":        item.SQFileHash,
				"file_hash":      item.FileHash,
				"res_hash":       item.ResFileHash,
				"mv_hash":        item.MvHash,
				"hq_hash":        item.HQFileHash,
				"audio_id":       formatKugouNumber(item.Audioid),
				"album_audio_id": firstKugouNonEmpty(formatKugouNumber(item.MixSongID), formatKugouNumber(item.ID)),
				"album_id":       item.AlbumID,
				"privilege":      strconv.Itoa(item.Privilege),
			},
		})
	}
	return songs, nil
}

type kugouSearchResponse struct {
	Data struct {
		Lists []kugouSearchItem `json:"lists"`
	} `json:"data"`
}

type kugouSearchItem struct {
	ID          interface{} `json:"ID"`
	MixSongID   interface{} `json:"MixSongID"`
	SongName    string      `json:"SongName"`
	SingerName  string      `json:"SingerName"`
	AlbumName   string      `json:"AlbumName"`
	AlbumID     string      `json:"AlbumID"`
	Audioid     interface{} `json:"Audioid"`
	Duration    int         `json:"Duration"`
	FileHash    string      `json:"FileHash"`
	SQFileHash  string      `json:"SQFileHash"`
	HQFileHash  string      `json:"HQFileHash"`
	ResFileHash string      `json:"ResFileHash"`
	MvHash      string      `json:"MvHash"`
	SQFileSize  int64       `json:"SQFileSize"`
	HQFileSize  int64       `json:"HQFileSize"`
	ResFileSize int64       `json:"ResFileSize"`
	FileSize    interface{} `json:"FileSize"`
	Image       string      `json:"Image"`
	Privilege   int         `json:"Privilege"`
	TransParam  struct {
		Ogg320Hash     string `json:"ogg_320_hash"`
		Ogg128Hash     string `json:"ogg_128_hash"`
		Ogg320FileSize int64  `json:"ogg_320_filesize"`
		Ogg128FileSize int64  `json:"ogg_128_filesize"`
	} `json:"TransParam"`
}

func kugouItemSize(item kugouSearchItem, hash string) int64 {
	var size int64
	switch value := item.FileSize.(type) {
	case float64:
		size = int64(value)
	case string:
		size, _ = strconv.ParseInt(value, 10, 64)
	}
	switch hash {
	case item.SQFileHash:
		if item.SQFileSize > 0 {
			size = item.SQFileSize
		}
	case item.HQFileHash:
		if item.HQFileSize > 0 {
			size = item.HQFileSize
		}
	case item.ResFileHash:
		if item.ResFileSize > 0 {
			size = item.ResFileSize
		}
	case item.TransParam.Ogg320Hash:
		if item.TransParam.Ogg320FileSize > 0 {
			size = item.TransParam.Ogg320FileSize
		}
	case item.TransParam.Ogg128Hash:
		if item.TransParam.Ogg128FileSize > 0 {
			size = item.TransParam.Ogg128FileSize
		}
	}
	return size
}

func cleanKugouText(value string) string {
	value = html.UnescapeString(value)
	value = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(value, "")
	return strings.TrimSpace(value)
}

func firstKugouNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func formatKugouNumber(value interface{}) string {
	switch number := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(number)
	case float64:
		return strconv.FormatFloat(number, 'f', 0, 64)
	case int:
		return strconv.Itoa(number)
	case int64:
		return strconv.FormatInt(number, 10)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
