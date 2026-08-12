package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/model"
	"github.com/hezebin/media-dl/internal/util"
)

// Options 下载选项。
type Options struct {
	OutputDir     string
	Filename      string // 不含扩展名；空则用标题
	Format        string // 容器格式，默认 mp4
	DownloadCover bool
	Quiet         bool
}

// Result 下载结果路径。
type Result struct {
	VideoPath string
	CoverPath string
}

func Download(client *httpx.Client, info *model.VideoInfo, opt Options) (*Result, error) {
	if opt.OutputDir == "" {
		opt.OutputDir = "."
	}
	if opt.Format == "" {
		opt.Format = "mp4"
	}
	opt.Format = strings.TrimPrefix(strings.ToLower(opt.Format), ".")
	if err := os.MkdirAll(opt.OutputDir, 0o755); err != nil {
		return nil, err
	}

	base := opt.Filename
	if base == "" {
		base = util.SanitizeFilename(info.Title)
		if base == "" {
			base = info.ID
		}
	}
	base = util.SanitizeFilename(base)

	fmtSel := info.BestFormat()
	if fmtSel == nil {
		return nil, fmt.Errorf("没有可下载格式")
	}

	out := &Result{}
	videoPath := filepath.Join(opt.OutputDir, base+"."+opt.Format)

	// DASH 分离流：AudioURL 非空时需 ffmpeg 合并
	needMerge := fmtSel.AudioURL != ""

	if needMerge {
		tmpVideo := filepath.Join(opt.OutputDir, base+".video.tmp")
		tmpAudio := filepath.Join(opt.OutputDir, base+".audio.tmp")
		defer os.Remove(tmpVideo)
		defer os.Remove(tmpAudio)

		if !opt.Quiet {
			fmt.Fprintf(os.Stderr, "下载视频流...\n")
		}
		if err := saveURL(client, fmtSel.URL, tmpVideo, fmtSel.Headers); err != nil {
			return nil, fmt.Errorf("下载视频流失败: %w", err)
		}
		if !opt.Quiet {
			fmt.Fprintf(os.Stderr, "下载音频流...\n")
		}
		if err := saveURL(client, fmtSel.AudioURL, tmpAudio, fmtSel.Headers); err != nil {
			return nil, fmt.Errorf("下载音频流失败: %w", err)
		}
		if !opt.Quiet {
			fmt.Fprintf(os.Stderr, "合并为 %s (需要 ffmpeg)...\n", opt.Format)
		}
		if err := mergeAV(tmpVideo, tmpAudio, videoPath, opt.Format); err != nil {
			return nil, err
		}
	} else {
		if !opt.Quiet {
			fmt.Fprintf(os.Stderr, "下载中 -> %s\n", videoPath)
		}
		tmp := videoPath + ".part"
		if err := saveURL(client, fmtSel.URL, tmp, fmtSel.Headers); err != nil {
			return nil, err
		}
		// 若源扩展名与目标不同，尝试 remux
		srcExt := fmtSel.Ext
		if srcExt == "" {
			srcExt = "mp4"
		}
		if srcExt != opt.Format {
			if err := remux(tmp, videoPath, opt.Format); err != nil {
				// remux 失败则直接改名保留
				_ = os.Rename(tmp, videoPath)
			} else {
				_ = os.Remove(tmp)
			}
		} else {
			if err := os.Rename(tmp, videoPath); err != nil {
				return nil, err
			}
		}
	}
	out.VideoPath = videoPath

	if opt.DownloadCover && info.CoverURL != "" {
		coverPath := filepath.Join(opt.OutputDir, base+".jpg")
		if !opt.Quiet {
			fmt.Fprintf(os.Stderr, "下载封面 -> %s\n", coverPath)
		}
		headers := map[string]string{}
		if fmtSel.Headers != nil {
			for k, v := range fmtSel.Headers {
				headers[k] = v
			}
		}
		if err := saveURL(client, info.CoverURL, coverPath, headers); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 封面下载失败: %v\n", err)
		} else {
			out.CoverPath = coverPath
		}
	}
	return out, nil
}

func saveURL(client *httpx.Client, rawURL, path string, headers map[string]string) error {
	resp, err := client.Get(rawURL, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 32*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if total > 0 {
				pct := float64(written) * 100 / float64(total)
				fmt.Fprintf(os.Stderr, "\r  %.1f%% (%s / %s)", pct, human(written), human(total))
			} else {
				fmt.Fprintf(os.Stderr, "\r  %s", human(written))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	fmt.Fprintln(os.Stderr)
	return nil
}

func human(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func mergeAV(video, audio, out, format string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("检测到音视频分离流，需要系统安装 ffmpeg 才能合并。也可尝试其它清晰度")
	}
	args := []string{
		"-y", "-i", video, "-i", audio,
		"-c", "copy",
		"-map", "0:v:0", "-map", "1:a:0",
	}
	switch format {
	case "mp4", "m4v":
		args = append(args, "-movflags", "+faststart")
	}
	args = append(args, out)
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg 合并失败: %w", err)
	}
	return nil
}

func remux(in, out, format string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return err
	}
	args := []string{"-y", "-i", in, "-c", "copy"}
	if format == "mp4" {
		args = append(args, "-movflags", "+faststart")
	}
	args = append(args, out)
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// LoadCookiesFile 支持 Netscape cookies.txt 的简化解析（name/value/domain）。
func LoadCookiesFile(path string) ([]*http.Cookie, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cookies []*http.Cookie
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Netscape: domain flag path secure expiry name value
		fields := strings.Split(line, "\t")
		if len(fields) >= 7 {
			cookies = append(cookies, &http.Cookie{
				Name:   fields[5],
				Value:  fields[6],
				Domain: strings.TrimPrefix(fields[0], "#HttpOnly_"),
				Path:   fields[2],
			})
			continue
		}
		// 简单 name=value; 多行或整行 header
		if strings.Contains(line, "=") && !strings.Contains(line, "\t") {
			cookies = append(cookies, httpx.ParseCookieHeader(line, "")...)
		}
	}
	return cookies, nil
}
