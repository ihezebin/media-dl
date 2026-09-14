package music

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	musicmodel "github.com/guohuiyuan/music-lib/model"
	"github.com/hezebin/media-dl/internal/httpx"
)

const searchProbeWorkers = 4

// validateSearchResults keeps the requested first page, marks entries whose
// real download URL cannot be reached, and appends the same number of
// candidates from the provider's remaining search results.
func (s *Service) validateSearchResults(provider Provider, songs []musicmodel.Song, limit int, cookie string) ([]musicmodel.Song, error) {
	if len(songs) == 0 {
		return songs, nil
	}
	firstCount := len(songs)
	if firstCount > limit {
		firstCount = limit
	}
	first := append([]musicmodel.Song(nil), songs[:firstCount]...)
	client, err := httpx.New(httpx.Options{Timeout: 10 * time.Second})
	if err != nil {
		return first, nil
	}

	checkedFirst, invalidCount := s.validateSongs(provider, first, client, cookie)
	result := checkedFirst
	if invalidCount == 0 || firstCount == len(songs) {
		return result, nil
	}

	candidateCount := invalidCount
	available := len(songs) - firstCount
	if candidateCount > available {
		candidateCount = available
	}
	candidates, _ := s.validateSongs(provider, songs[firstCount:firstCount+candidateCount], client, cookie)
	return append(result, candidates...), nil
}

func (s *Service) validateSongs(provider Provider, songs []musicmodel.Song, client *httpx.Client, cookie string) ([]musicmodel.Song, int) {
	if len(songs) == 0 {
		return songs, 0
	}
	workers := searchProbeWorkers
	if len(songs) < workers {
		workers = len(songs)
	}

	checked := make([]musicmodel.Song, len(songs))
	jobs := make(chan int)
	var wg sync.WaitGroup
	var invalidCount int
	var invalidMu sync.Mutex
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				song := songs[index]
				if downloadURL, err := s.resolvePlayableURL(provider, &song, client, cookie); err == nil {
					song.URL = downloadURL
					song.IsInvalid = false
					deleteExtra(&song, "invalid_reason")
				} else {
					song.URL = ""
					song.IsInvalid = true
					setExtra(&song, "invalid_reason", trimReason(err.Error()))
					invalidMu.Lock()
					invalidCount++
					invalidMu.Unlock()
				}
				checked[index] = song
			}
		}()
	}
	for index := range songs {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	return checked, invalidCount
}

func (s *Service) resolvePlayableURL(provider Provider, song *musicmodel.Song, client *httpx.Client, cookie string) (string, error) {
	urls := make([]string, 0, 3)
	if value := strings.TrimSpace(song.URL); value != "" {
		urls = append(urls, value)
	}
	if provider.GetDownloadURL != nil {
		if value, err := provider.GetDownloadURL(song); err == nil && strings.TrimSpace(value) != "" {
			urls = append(urls, value)
		}
	}
	if provider.GetDownloadURLFallback != nil {
		if value, err := provider.GetDownloadURLFallback(song); err == nil && strings.TrimSpace(value) != "" {
			urls = append(urls, value)
		}
	}
	if len(urls) == 0 {
		return "", fmt.Errorf("没有可用下载地址")
	}

	seen := make(map[string]struct{}, len(urls))
	var reasons []string
	for _, rawURL := range urls {
		if _, ok := seen[rawURL]; ok {
			continue
		}
		seen[rawURL] = struct{}{}
		status, err := probeDownloadURL(client, rawURL, cookie, provider.Name)
		if err == nil {
			return rawURL, nil
		}
		if status > 0 {
			reasons = append(reasons, "HTTP "+strconv.Itoa(status))
		} else {
			reasons = append(reasons, err.Error())
		}
	}
	return "", fmt.Errorf("下载地址不可用（%s）", strings.Join(uniqueStrings(reasons), ", "))
}

func probeDownloadURL(client *httpx.Client, rawURL, cookie, platform string) (int, error) {
	attempts := make([]map[string]string, 0, 4)
	for _, attempt := range []struct {
		cookie string
		range_ bool
	}{
		{cookie: cookie, range_: true},
		{cookie: "", range_: true},
		{cookie: cookie, range_: false},
		{cookie: "", range_: false},
	} {
		headers := mediaRequestHeaders(platform, attempt.cookie)
		if attempt.range_ {
			headers["Range"] = "bytes=0-1"
		}
		attempts = append(attempts, headers)
	}

	lastStatus := 0
	var lastErr error
	for _, headers := range attempts {
		resp, err := client.Get(rawURL, headers)
		if err != nil {
			lastErr = err
			continue
		}
		status := resp.StatusCode
		_, _ = io.CopyN(io.Discard, resp.Body, 4096)
		resp.Body.Close()
		if status == http.StatusOK || status == http.StatusPartialContent {
			return status, nil
		}
		lastStatus = status
		lastErr = fmt.Errorf("unexpected status")
	}
	if lastStatus > 0 {
		return lastStatus, lastErr
	}
	return 0, lastErr
}

func mediaRequestHeaders(platform, cookie string) map[string]string {
	headers := map[string]string{
		"Accept":     "*/*",
		"User-Agent": httpx.DefaultUA,
	}
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "netease":
		headers["Referer"] = "http://music.163.com/"
		headers["Origin"] = "http://music.163.com"
	case "kugou":
		headers["Referer"] = "https://www.kugou.com/"
	case "qq":
		headers["Referer"] = "https://y.qq.com/"
	case "kuwo":
		headers["Referer"] = "https://www.kuwo.cn/"
	case "migu":
		headers["Referer"] = "https://music.migu.cn/"
	}
	if strings.TrimSpace(cookie) != "" {
		headers["Cookie"] = cookie
	}
	return headers
}

func setExtra(song *musicmodel.Song, key, value string) {
	if song.Extra == nil {
		song.Extra = map[string]string{}
	}
	song.Extra[key] = value
}

func deleteExtra(song *musicmodel.Song, key string) {
	if song.Extra != nil {
		delete(song.Extra, key)
	}
}

func trimReason(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 180 {
		return value[:180] + "..."
	}
	return value
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok || strings.TrimSpace(value) == "" {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (s *Service) resolveDownloadURL(provider Provider, song *musicmodel.Song) (string, error) {
	if provider.GetDownloadURL != nil {
		if value, err := provider.GetDownloadURL(song); err == nil && strings.TrimSpace(value) != "" {
			return value, nil
		} else if provider.GetDownloadURLFallback == nil && err != nil {
			return "", err
		}
	}
	if provider.GetDownloadURLFallback != nil {
		if value, err := provider.GetDownloadURLFallback(song); err == nil && strings.TrimSpace(value) != "" {
			return value, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("平台返回空下载地址")
}
