package model

import "encoding/json"

// VideoInfo 统一的视频元信息，供 info / download 共用。
type VideoInfo struct {
	Success     bool     `json:"success"`
	ErrMsg      string   `json:"err_msg,omitempty"` // 仅失败时出现
	Platform    string   `json:"platform"`
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	AuthorID    string   `json:"author_id"`
	Duration    float64  `json:"duration"`
	CoverURL    string   `json:"cover_url"`
	WebpageURL  string   `json:"webpage_url"`
	VideoURL    string   `json:"video_url"` // 优选下载地址（由 Normalize 填充）
	Formats     []Format `json:"formats"`
}

// Format 可下载的媒体流（字段固定，便于三平台统一 JSON）。
type Format struct {
	FormatID   string            `json:"format_id"`
	URL        string            `json:"url"`
	Ext        string            `json:"ext"`
	Quality    string            `json:"quality"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	Filesize   int64             `json:"filesize"`
	VCodec     string            `json:"vcodec"`
	ACodec     string            `json:"acodec"`
	HasAudio   bool              `json:"has_audio"`
	HasVideo   bool              `json:"has_video"`
	AudioURL   string            `json:"audio_url"` // DASH 分离音轨；空表示一体流
	Headers    map[string]string `json:"-"`
	Preference int               `json:"-"`
	Protocol   string            `json:"-"` // "m3u8" 表示 HLS，下载需 ffmpeg
	PartURLs   []string          `json:"-"` // 多段 URL，下载后拼接
}

// Normalize 补齐统一字段（空切片、优选 video_url、success）。
func (v *VideoInfo) Normalize() {
	if v.Formats == nil {
		v.Formats = []Format{}
	}
	if v.ErrMsg != "" {
		v.Success = false
		return
	}
	v.Success = true
	if best := v.BestFormat(); best != nil {
		v.VideoURL = best.URL
	}
}

// Fail 构造解析失败时的 JSON 对象（含具体原因）。
func Fail(platform, webpageURL string, err error) *VideoInfo {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if msg == "" {
		msg = "未知错误"
	}
	return &VideoInfo{
		Success:    false,
		ErrMsg:     msg,
		Platform:   platform,
		WebpageURL: webpageURL,
		Formats:    []Format{},
	}
}

// MarshalInfo 输出固定字段的 JSON。
func (v *VideoInfo) MarshalInfo() ([]byte, error) {
	v.Normalize()
	return json.MarshalIndent(v, "", "  ")
}

// BestFormat 选择综合最优格式（优先完整音视频一体流）。
func (v *VideoInfo) BestFormat() *Format {
	if len(v.Formats) == 0 {
		return nil
	}
	best := &v.Formats[0]
	for i := range v.Formats {
		f := &v.Formats[i]
		if formatScore(f) > formatScore(best) {
			best = f
		}
	}
	return best
}

func formatScore(f *Format) int {
	score := f.Preference
	if f.HasVideo && f.HasAudio && f.AudioURL == "" {
		score += 10000
	}
	if f.HasVideo && f.AudioURL != "" {
		score += 5000
	}
	score += f.Width*f.Height/1000 + int(f.Filesize/1024/1024)
	return score
}
