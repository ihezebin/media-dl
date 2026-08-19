package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/hezebin/media-dl/internal/downloader"
	"github.com/hezebin/media-dl/internal/extractor"
	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/model"

	_ "github.com/hezebin/media-dl/internal/extractor/bilibili"
	_ "github.com/hezebin/media-dl/internal/extractor/douyin"
	_ "github.com/hezebin/media-dl/internal/extractor/iqiyi"
	_ "github.com/hezebin/media-dl/internal/extractor/tencent"
	_ "github.com/hezebin/media-dl/internal/extractor/weibo"
	_ "github.com/hezebin/media-dl/internal/extractor/xiaohongshu"
	_ "github.com/hezebin/media-dl/internal/extractor/xigua"
	_ "github.com/hezebin/media-dl/internal/extractor/youku"
)

var (
	flagOutput  string
	flagFormat  string
	flagName    string
	flagCover   bool
	flagCookies string
	flagCookie  string
	flagProxy   string
)

func main() {
	root := &cobra.Command{
		Version:       "1.0.0",
		Use:           "media-dl",
		Short:         "解析并下载多平台视频",
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `media-dl 从分享链接（含短链）解析视频信息并下载。

必须显式指定平台: douyin | bilibili | xiaohongshu | weibo | youku | iqiyi | xigua | tencent
（别名: dy / bili / xhs / wb / yk / iq / 西瓜 / qq）

示例:
  media-dl info douyin "https://v.douyin.com/xxx"
  media-dl download bilibili "https://www.bilibili.com/video/BVxxx" -f mp4
  media-dl dl xhs "https://www.xiaohongshu.com/explore/xxx" --cover
  media-dl info tencent "https://v.qq.com/x/page/xxx.html"`,
	}

	root.PersistentFlags().StringVar(&flagProxy, "proxy", "", "HTTP/HTTPS 代理，如 http://127.0.0.1:7890")
	root.PersistentFlags().StringVar(&flagCookies, "cookies", "", "Netscape cookies.txt 路径（B 站 412 / 小红书 / 西瓜 / 腾讯 VIP 等建议）")
	root.PersistentFlags().StringVar(&flagCookie, "cookie", "", "直接传入 Cookie 头字符串")

	infoCmd := &cobra.Command{
		Use:   "info <platform> <url>",
		Short: "只解析视频信息，输出统一 JSON（不下载）",
		Args:  cobra.ExactArgs(2),
		RunE:  runInfo,
	}

	dlCmd := &cobra.Command{
		Use:     "download <platform> <url>",
		Aliases: []string{"dl"},
		Short:   "解析并下载视频",
		Args:    cobra.ExactArgs(2),
		RunE:    runDownload,
	}
	dlCmd.Flags().StringVarP(&flagOutput, "output", "o", ".", "保存目录")
	dlCmd.Flags().StringVarP(&flagFormat, "format", "f", "mp4", "输出容器格式 (mp4/mkv/...)")
	dlCmd.Flags().StringVarP(&flagName, "name", "n", "", "输出文件名（不含扩展名）")
	dlCmd.Flags().BoolVar(&flagCover, "cover", false, "同时下载封面图")

	root.AddCommand(infoCmd, dlCmd)
	if err := root.Execute(); err != nil {
		var printed jsonPrintedError
		if !errors.As(err, &printed) {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}

// jsonPrintedError 表示失败 JSON 已写到 stdout，无需再往 stderr 打 Error:。
type jsonPrintedError struct{ err error }

func (e jsonPrintedError) Error() string { return e.err.Error() }
func (e jsonPrintedError) Unwrap() error { return e.err }

func newClient() (*httpx.Client, error) {
	var cookies []*http.Cookie
	if flagCookies != "" {
		cs, err := downloader.LoadCookiesFile(flagCookies)
		if err != nil {
			return nil, fmt.Errorf("读取 cookies 文件失败: %w", err)
		}
		cookies = append(cookies, cs...)
	}
	if flagCookie != "" {
		for _, domain := range []string{
			"douyin.com", "bilibili.com", "xiaohongshu.com",
			"weibo.com", "weibo.cn", "youku.com", "tudou.com",
			"iqiyi.com", "iq.com", "ixigua.com", "toutiao.com", "qq.com",
		} {
			cookies = append(cookies, httpx.ParseCookieHeader(flagCookie, domain)...)
		}
	}
	return httpx.New(httpx.Options{
		Proxy:   flagProxy,
		Cookies: cookies,
	})
}

func runInfo(_ *cobra.Command, args []string) error {
	platform, rawURL := args[0], args[1]
	canon, _ := extractor.NormalizePlatform(platform)
	if canon == "" {
		canon = platform
	}
	printJSON := func(info *model.VideoInfo) error {
		b, err := info.MarshalInfo()
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	client, err := newClient()
	if err != nil {
		if perr := printJSON(model.Fail(canon, rawURL, err)); perr != nil {
			return perr
		}
		return jsonPrintedError{err}
	}
	info, err := extractor.Extract(client, platform, rawURL)
	if err != nil {
		if perr := printJSON(model.Fail(canon, rawURL, err)); perr != nil {
			return perr
		}
		return jsonPrintedError{err}
	}
	if err := printJSON(info); err != nil {
		return err
	}
	return nil
}

func runDownload(_ *cobra.Command, args []string) error {
	platform, rawURL := args[0], args[1]
	client, err := newClient()
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "解析中...")
	info, err := extractor.Extract(client, platform, rawURL)
	if err != nil {
		return err
	}
	printInfoBrief(info)
	fmt.Fprintln(os.Stderr)

	res, err := downloader.Download(client, info, downloader.Options{
		OutputDir:     flagOutput,
		Filename:      flagName,
		Format:        flagFormat,
		DownloadCover: flagCover,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "完成: %s\n", res.VideoPath)
	if res.CoverPath != "" {
		fmt.Fprintf(os.Stderr, "封面: %s\n", res.CoverPath)
	}
	return nil
}

func printInfoBrief(info *model.VideoInfo) {
	fmt.Fprintf(os.Stderr, "平台: %s\n", info.Platform)
	fmt.Fprintf(os.Stderr, "ID:   %s\n", info.ID)
	fmt.Fprintf(os.Stderr, "标题: %s\n", info.Title)
	if info.Author != "" {
		fmt.Fprintf(os.Stderr, "作者: %s\n", info.Author)
	}
	if info.VideoURL != "" {
		fmt.Fprintf(os.Stderr, "视频: %s\n", info.VideoURL)
	}
}
