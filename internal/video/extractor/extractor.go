package extractor

import (
	"fmt"
	"strings"

	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/video/model"
)

// Extractor 各平台解析器接口。
type Extractor interface {
	Name() string
	Match(rawURL string) bool
	Extract(client *httpx.Client, rawURL string) (*model.VideoInfo, error)
}

var registry []Extractor

func Register(e Extractor) {
	registry = append(registry, e)
}

func All() []Extractor { return registry }

// NormalizePlatform 规范化平台名（含别名）。
func NormalizePlatform(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "douyin", "dy", "抖音":
		return "douyin", nil
	case "bilibili", "bili", "b23", "哔哩哔哩", "哔站", "b站":
		return "bilibili", nil
	case "xiaohongshu", "xhs", "redbook", "小红书":
		return "xiaohongshu", nil
	case "weibo", "wb", "微博":
		return "weibo", nil
	case "youku", "yk", "优酷":
		return "youku", nil
	case "iqiyi", "iq", "爱奇艺":
		return "iqiyi", nil
	case "xigua", "ixigua", "西瓜", "西瓜视频":
		return "xigua", nil
	case "tencent", "qq", "vqq", "腾讯", "腾讯视频":
		return "tencent", nil
	case "youtube", "yt", "油管":
		return "youtube", nil
	case "tiktok", "tk", "抖音国际版":
		return "tiktok", nil
	case "kuaishou", "ks", "快手", "kwai":
		return "kuaishou", nil
	case "baidu", "haokan", "百度视频", "好看视频":
		return "baidu", nil
	case "twitter", "x", "推特":
		return "twitter", nil
	case "douyu", "斗鱼":
		return "douyu", nil
	case "huya", "虎牙":
		return "huya", nil
	default:
		return "", fmt.Errorf("未知平台 %q，可选: douyin | bilibili | xiaohongshu | weibo | youku | iqiyi | xigua | tencent | youtube | tiktok | kuaishou | baidu | twitter | douyu | huya", name)
	}
}

func byName(name string) (Extractor, error) {
	canon, err := NormalizePlatform(name)
	if err != nil {
		return nil, err
	}
	for _, e := range registry {
		if e.Name() == canon {
			return e, nil
		}
	}
	return nil, fmt.Errorf("平台未注册: %s", canon)
}

// Extract 使用指定平台解析 URL（必须显式指定平台）。
func Extract(client *httpx.Client, platform, rawURL string) (*model.VideoInfo, error) {
	e, err := byName(platform)
	if err != nil {
		return nil, err
	}
	info, err := e.Extract(client, rawURL)
	if err != nil {
		return nil, fmt.Errorf("[%s] %w", e.Name(), err)
	}
	info.Normalize()
	return info, nil
}
