package music

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	musicmodel "github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/soda"
	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/util"
)

type DownloadOptions struct {
	OutputDir      string
	Filename       string
	DownloadCover  bool
	DownloadLyrics bool
}

type DownloadResult struct {
	AudioPath  string `json:"audio_path"`
	CoverPath  string `json:"cover_path,omitempty"`
	LyricsPath string `json:"lyrics_path,omitempty"`
}

// AssetOptions 控制 HTTP API 单独下载歌曲、封面或歌词。
type AssetOptions struct {
	Action    string
	OutputDir string
	Filename  string
}

// DownloadAsset 下载单个音乐资源，避免网页端点击歌词或封面时重复下载音频。
func (s *Service) DownloadAsset(client *httpx.Client, song *musicmodel.Song, opt AssetOptions) (*DownloadResult, error) {
	if song == nil {
		return nil, fmt.Errorf("歌曲信息为空")
	}
	if opt.OutputDir == "" {
		opt.OutputDir = "."
	}
	if err := os.MkdirAll(opt.OutputDir, 0o755); err != nil {
		return nil, err
	}
	base := strings.TrimSpace(opt.Filename)
	if base == "" {
		base = strings.TrimSpace(song.Name)
		if song.Artist != "" {
			base += " - " + strings.TrimSpace(song.Artist)
		}
	}
	base = util.SanitizeFilename(base)

	switch strings.ToLower(strings.TrimSpace(opt.Action)) {
	case "audio", "song", "music":
		return s.Download(client, song, DownloadOptions{OutputDir: opt.OutputDir, Filename: base})
	case "cover":
		if strings.TrimSpace(song.Cover) == "" {
			return nil, fmt.Errorf("该歌曲没有封面地址")
		}
		if client == nil {
			var err error
			client, err = httpx.New(httpx.Options{})
			if err != nil {
				return nil, err
			}
		}
		path := filepath.Join(opt.OutputDir, base+".jpg")
		if err := saveURL(client, song.Cover, path, song.Source, s.cookie); err != nil {
			return nil, fmt.Errorf("下载封面失败: %w", err)
		}
		return &DownloadResult{CoverPath: path}, nil
	case "lyrics", "lyric":
		provider, err := s.provider(song.Source)
		if err != nil {
			return nil, err
		}
		lyrics, err := provider.GetLyrics(song)
		if err != nil {
			return nil, fmt.Errorf("获取歌词失败: %w", err)
		}
		if strings.TrimSpace(lyrics) == "" {
			return nil, fmt.Errorf("该歌曲暂无歌词")
		}
		path := filepath.Join(opt.OutputDir, base+".lrc")
		if err := os.WriteFile(path, []byte(lyrics), 0o644); err != nil {
			return nil, fmt.Errorf("保存歌词失败: %w", err)
		}
		return &DownloadResult{LyricsPath: path}, nil
	default:
		return nil, fmt.Errorf("不支持的资源类型 %q，可选 audio、cover、lyrics", opt.Action)
	}
}

func (s *Service) Download(client *httpx.Client, song *musicmodel.Song, opt DownloadOptions) (*DownloadResult, error) {
	if song == nil {
		return nil, fmt.Errorf("歌曲信息为空")
	}
	if strings.TrimSpace(song.Source) == "" {
		return nil, fmt.Errorf("歌曲来源为空")
	}
	if client == nil {
		var err error
		client, err = httpx.New(httpx.Options{})
		if err != nil {
			return nil, err
		}
	}
	if opt.OutputDir == "" {
		opt.OutputDir = "."
	}
	if err := os.MkdirAll(opt.OutputDir, 0o755); err != nil {
		return nil, err
	}

	provider, err := s.provider(song.Source)
	if err != nil {
		return nil, err
	}
	ext := normalizeAudioExt(song.Ext)
	if ext == "" {
		ext = "mp3"
	}
	base := strings.TrimSpace(opt.Filename)
	if base == "" {
		base = strings.TrimSpace(song.Name)
		if song.Artist != "" {
			base += " - " + strings.TrimSpace(song.Artist)
		}
	}
	base = util.SanitizeFilename(base)
	audioPath := filepath.Join(opt.OutputDir, base+"."+ext)

	fmt.Fprintf(os.Stderr, "获取下载地址...\n")
	if song.Source == "soda" {
		fmt.Fprintf(os.Stderr, "下载并解密汽水音乐 -> %s\n", audioPath)
		if err := soda.New(s.cookie).Download(song, audioPath); err != nil {
			return nil, fmt.Errorf("汽水音乐下载失败: %w", err)
		}
	} else {
		audioURL, err := s.resolveDownloadURL(provider, song)
		if err != nil {
			return nil, fmt.Errorf("获取下载地址失败: %w", err)
		}
		if strings.TrimSpace(audioURL) == "" {
			return nil, fmt.Errorf("平台返回空下载地址")
		}
		fmt.Fprintf(os.Stderr, "下载音频 -> %s\n", audioPath)
		if err := saveURL(client, audioURL, audioPath, song.Source, s.cookie); err != nil {
			return nil, fmt.Errorf("下载音频失败: %w", err)
		}
	}

	result := &DownloadResult{AudioPath: audioPath}
	if opt.DownloadCover && strings.TrimSpace(song.Cover) != "" {
		coverPath := filepath.Join(opt.OutputDir, base+".jpg")
		if err := saveURL(client, song.Cover, coverPath, song.Source, s.cookie); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 封面下载失败: %v\n", err)
		} else {
			result.CoverPath = coverPath
		}
	}
	if opt.DownloadLyrics {
		lyrics, lyricErr := provider.GetLyrics(song)
		if lyricErr != nil {
			fmt.Fprintf(os.Stderr, "警告: 歌词获取失败: %v\n", lyricErr)
		} else if strings.TrimSpace(lyrics) != "" {
			lyricsPath := filepath.Join(opt.OutputDir, base+".lrc")
			if err := os.WriteFile(lyricsPath, []byte(lyrics), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "警告: 歌词保存失败: %v\n", err)
			} else {
				result.LyricsPath = lyricsPath
			}
		}
	}
	return result, nil
}

func normalizeAudioExt(ext string) string {
	ext = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ext)), ".")
	switch ext {
	case "mp3", "flac", "m4a", "aac", "ogg", "opus", "wav", "wma":
		return ext
	default:
		return ""
	}
}

func saveURL(client *httpx.Client, rawURL, path, platform, cookie string) error {
	resp, err := client.Get(rawURL, mediaRequestHeaders(platform, cookie))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}
