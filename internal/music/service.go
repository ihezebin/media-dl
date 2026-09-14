package music

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/guohuiyuan/music-lib/apple"
	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	musicmodel "github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
	"github.com/hezebin/media-dl/internal/httpx"
)

// PlatformNames 是 CLI 支持的平台及其默认搜索顺序。
var PlatformNames = []string{
	"netease",
	"qq",
	"kugou",
	"kuwo",
	"migu",
	"fivesing",
	"qianqian",
	"soda",
	"bilibili",
	"apple",
}

var platformAliases = map[string]string{
	"netease":     "netease",
	"163":         "netease",
	"网易云":         "netease",
	"网易云音乐":       "netease",
	"qq":          "qq",
	"qqmusic":     "qq",
	"qq音乐":        "qq",
	"腾讯音乐":        "qq",
	"kugou":       "kugou",
	"酷狗":          "kugou",
	"酷狗音乐":        "kugou",
	"kuwo":        "kuwo",
	"酷我":          "kuwo",
	"酷我音乐":        "kuwo",
	"migu":        "migu",
	"咪咕":          "migu",
	"咪咕音乐":        "migu",
	"fivesing":    "fivesing",
	"5sing":       "fivesing",
	"qianqian":    "qianqian",
	"千千":          "qianqian",
	"千千音乐":        "qianqian",
	"soda":        "soda",
	"汽水":          "soda",
	"汽水音乐":        "soda",
	"bilibili":    "bilibili",
	"bili":        "bilibili",
	"哔哩哔哩":        "bilibili",
	"apple":       "apple",
	"applemusic":  "apple",
	"apple music": "apple",
	"苹果音乐":        "apple",
}

// Provider 将 music-lib 的平台实例统一为 CLI 所需的歌曲能力。
type Provider struct {
	Name           string
	Search         func(string) ([]musicmodel.Song, error)
	Parse          func(string) (*musicmodel.Song, error)
	GetDownloadURL func(*musicmodel.Song) (string, error)
	GetLyrics      func(*musicmodel.Song) (string, error)
}

// Service 是音乐搜索、链接解析和下载的统一入口。
type Service struct {
	cookie         string
	appleTokenOnce sync.Once
	appleCookie    string
	appleTokenErr  error
}

func New(cookie string) *Service {
	return &Service{cookie: strings.TrimSpace(cookie)}
}

// NormalizePlatform 规范化平台名。
func NormalizePlatform(name string) (string, error) {
	canon, ok := platformAliases[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return "", fmt.Errorf("未知音乐平台 %q，可选: %s", name, strings.Join(PlatformNames, " | "))
	}
	return canon, nil
}

func (s *Service) provider(name string) (Provider, error) {
	canon, err := NormalizePlatform(name)
	if err != nil {
		return Provider{}, err
	}

	switch canon {
	case "netease":
		p := netease.New(s.cookie)
		return Provider{canon, p.Search, p.Parse, p.GetDownloadURL, p.GetLyrics}, nil
	case "qq":
		p := qq.New(s.cookie)
		return Provider{canon, p.Search, p.Parse, p.GetDownloadURL, p.GetLyrics}, nil
	case "kugou":
		p := kugou.New(s.cookie)
		return Provider{canon, p.Search, p.Parse, p.GetDownloadURL, p.GetLyrics}, nil
	case "kuwo":
		p := kuwo.New(s.cookie)
		return Provider{canon, p.Search, p.Parse, p.GetDownloadURL, p.GetLyrics}, nil
	case "migu":
		p := migu.New(s.cookie)
		return Provider{canon, p.Search, p.Parse, p.GetDownloadURL, p.GetLyrics}, nil
	case "fivesing":
		p := fivesing.New(s.cookie)
		return Provider{canon, p.Search, p.Parse, p.GetDownloadURL, p.GetLyrics}, nil
	case "qianqian":
		p := qianqian.New(s.cookie)
		return Provider{canon, p.Search, p.Parse, p.GetDownloadURL, p.GetLyrics}, nil
	case "soda":
		p := soda.New(s.cookie)
		return Provider{canon, p.Search, p.Parse, p.GetDownloadURL, p.GetLyrics}, nil
	case "bilibili":
		p := bilibili.New(s.cookie)
		return Provider{canon, p.Search, p.Parse, p.GetDownloadURL, p.GetLyrics}, nil
	case "apple":
		return Provider{
			Name: canon,
			Search: func(keyword string) ([]musicmodel.Song, error) {
				p, err := s.appleProvider()
				if err != nil {
					return nil, err
				}
				return p.Search(keyword)
			},
			Parse: func(link string) (*musicmodel.Song, error) {
				p, err := s.appleProvider()
				if err != nil {
					return nil, err
				}
				return p.Parse(link)
			},
			GetDownloadURL: func(song *musicmodel.Song) (string, error) {
				p, err := s.appleProvider()
				if err != nil {
					return "", err
				}
				return p.GetDownloadURL(song)
			},
			GetLyrics: func(song *musicmodel.Song) (string, error) {
				p, err := s.appleProvider()
				if err != nil {
					return "", err
				}
				return p.GetLyrics(song)
			},
		}, nil
	default:
		return Provider{}, fmt.Errorf("平台未实现: %s", canon)
	}
}

func (s *Service) appleProvider() (*apple.Apple, error) {
	cookie, err := s.appleCookieValue()
	if err != nil {
		return nil, err
	}
	return apple.New(cookie), nil
}

type SearchOptions struct {
	Keyword   string
	Artist    string
	Title     string
	Platforms []string
	Limit     int
}

type SearchResponse struct {
	Keyword   string              `json:"keyword"`
	Platforms []string            `json:"platforms"`
	Results   []musicmodel.Song   `json:"results"`
	Errors    []PlatformSearchErr `json:"errors,omitempty"`
}

type PlatformSearchErr struct {
	Platform string `json:"platform"`
	Error    string `json:"error"`
}

func (o SearchOptions) Query() string {
	parts := make([]string, 0, 3)
	for _, part := range []string{o.Keyword, o.Artist, o.Title} {
		if value := strings.TrimSpace(part); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " ")
}

// Search 并发搜索指定平台；平台为空时搜索全部支持的平台。
func (s *Service) Search(opts SearchOptions) (*SearchResponse, error) {
	keyword := opts.Query()
	if keyword == "" {
		return nil, fmt.Errorf("搜索关键词不能为空，请传入歌名、歌手或 --artist/--title")
	}

	platforms, err := normalizePlatforms(opts.Platforms)
	if err != nil {
		return nil, err
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}

	resp := &SearchResponse{
		Keyword:   keyword,
		Platforms: append([]string(nil), platforms...),
		Results:   make([]musicmodel.Song, 0),
	}

	type result struct {
		index int
		songs []musicmodel.Song
		err   error
	}
	ch := make(chan result, len(platforms))
	var wg sync.WaitGroup
	for i, platform := range platforms {
		wg.Add(1)
		go func(index int, name string) {
			defer wg.Done()
			p, providerErr := s.provider(name)
			if providerErr != nil {
				ch <- result{index: index, err: providerErr}
				return
			}
			songs, searchErr := p.Search(keyword)
			if len(songs) > limit {
				songs = songs[:limit]
			}
			for i := range songs {
				songs[i].Source = p.Name
			}
			ch <- result{index: index, songs: songs, err: searchErr}
		}(i, platform)
	}
	wg.Wait()
	close(ch)

	byIndex := make([]result, len(platforms))
	for item := range ch {
		byIndex[item.index] = item
	}
	for _, item := range byIndex {
		if item.err != nil {
			resp.Errors = append(resp.Errors, PlatformSearchErr{
				Platform: platforms[item.index],
				Error:    item.err.Error(),
			})
		}
		resp.Results = append(resp.Results, item.songs...)
	}
	if len(resp.Results) == 0 && len(resp.Errors) > 0 {
		return resp, fmt.Errorf("所有音乐平台搜索失败")
	}
	return resp, nil
}

func normalizePlatforms(platforms []string) ([]string, error) {
	if len(platforms) == 0 {
		return append([]string(nil), PlatformNames...), nil
	}
	out := make([]string, 0, len(platforms))
	seen := make(map[string]bool, len(platforms))
	for _, raw := range platforms {
		for _, part := range strings.Split(raw, ",") {
			if strings.TrimSpace(part) == "" {
				continue
			}
			canon, err := NormalizePlatform(part)
			if err != nil {
				return nil, err
			}
			if !seen[canon] {
				seen[canon] = true
				out = append(out, canon)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("至少指定一个音乐平台")
	}
	return out, nil
}

// Parse 解析单曲链接。酷狗的分享页和 mixsong 页会先转换为 hash 链接。
func (s *Service) Parse(client *httpx.Client, platform, rawURL string) (*musicmodel.Song, error) {
	p, err := s.provider(platform)
	if err != nil {
		return nil, err
	}
	if p.Name == "kugou" {
		rawURL, err = resolveKugouURL(client, rawURL)
		if err != nil {
			return nil, err
		}
	}
	return p.Parse(rawURL)
}

func (s *Service) Provider(platform string) (Provider, error) {
	return s.provider(platform)
}

// ConfigureProxy 适配 music-lib 使用 http.DefaultTransport 的实现方式。
func ConfigureProxy(proxy string) error {
	if strings.TrimSpace(proxy) == "" {
		return nil
	}
	parsed, err := url.Parse(proxy)
	if err != nil {
		return fmt.Errorf("invalid proxy: %w", err)
	}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return fmt.Errorf("无法配置音乐库代理: 默认 HTTP transport 类型不受支持")
	}
	transport := base.Clone()
	transport.Proxy = http.ProxyURL(parsed)
	http.DefaultTransport = transport
	return nil
}
