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
		audioURL, err := provider.GetDownloadURL(song)
		if err != nil {
			return nil, fmt.Errorf("获取下载地址失败: %w", err)
		}
		if strings.TrimSpace(audioURL) == "" {
			return nil, fmt.Errorf("平台返回空下载地址")
		}
		fmt.Fprintf(os.Stderr, "下载音频 -> %s\n", audioPath)
		if err := saveURL(client, audioURL, audioPath); err != nil {
			return nil, fmt.Errorf("下载音频失败: %w", err)
		}
	}

	result := &DownloadResult{AudioPath: audioPath}
	if opt.DownloadCover && strings.TrimSpace(song.Cover) != "" {
		coverPath := filepath.Join(opt.OutputDir, base+".jpg")
		if err := saveURL(client, song.Cover, coverPath); err != nil {
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

func saveURL(client *httpx.Client, rawURL, path string) error {
	resp, err := client.Get(rawURL, map[string]string{
		"Accept":     "*/*",
		"User-Agent": httpx.DefaultUA,
	})
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
