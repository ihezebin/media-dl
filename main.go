package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	apihttp "github.com/hezebin/media-dl/httpserver"
	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/music"
	"github.com/hezebin/media-dl/internal/video/downloader"
	"github.com/hezebin/media-dl/internal/video/extractor"
	"github.com/hezebin/media-dl/internal/video/model"

	_ "github.com/hezebin/media-dl/internal/video/extractor/bilibili"
	_ "github.com/hezebin/media-dl/internal/video/extractor/douyin"
	_ "github.com/hezebin/media-dl/internal/video/extractor/iqiyi"
	_ "github.com/hezebin/media-dl/internal/video/extractor/tencent"
	_ "github.com/hezebin/media-dl/internal/video/extractor/web"
	_ "github.com/hezebin/media-dl/internal/video/extractor/weibo"
	_ "github.com/hezebin/media-dl/internal/video/extractor/xiaohongshu"
	_ "github.com/hezebin/media-dl/internal/video/extractor/xigua"
	_ "github.com/hezebin/media-dl/internal/video/extractor/youku"
)

var (
	flagOutput  string
	flagFormat  string
	flagName    string
	flagCover   bool
	flagCookies string
	flagCookie  string
	flagProxy   string

	flagMusicPlatforms []string
	flagMusicArtist    string
	flagMusicTitle     string
	flagMusicLimit     int
	flagMusicOutput    string
	flagMusicName      string
	flagMusicCover     bool
	flagMusicLyrics    bool
)

func main() {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		var printed jsonPrintedError
		if !errors.As(err, &printed) {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Version:       "1.0.3",
		Use:           "media-dl",
		Short:         "解析并下载多平台视频和音乐",
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `media-dl 按视频和音乐两个业务域提供解析、搜索和下载能力。

视频:
  media-dl video info douyin "https://v.douyin.com/xxx"
  media-dl video download bilibili "https://www.bilibili.com/video/BVxxx" -f mp4

音乐:
  media-dl music search --artist "周杰伦" --title "晴天"
  media-dl music download netease "https://music.163.com/#/song?id=123456"

服务:
  media-dl server --port 8080 --web-dir ./webui/dist`,
	}

	root.PersistentFlags().StringVar(&flagProxy, "proxy", envOrDefault("MEDIA_DL_PROXY", ""), "video/music 共用的 HTTP/HTTPS 代理，如 http://127.0.0.1:7890")
	root.PersistentFlags().StringVar(&flagCookies, "cookies", envOrDefault("MEDIA_DL_COOKIES", ""), "video/music 共用的 Netscape cookies.txt 路径")
	root.PersistentFlags().StringVar(&flagCookie, "cookie", envOrDefault("MEDIA_DL_COOKIE", ""), "video/music 共用的直接 Cookie 头字符串")

	root.AddCommand(newVideoCommand(), newMusicCommand(), newServerCommand())
	return root
}

func newServerCommand() *cobra.Command {
	var port uint
	var webDir = envOrDefault("MEDIA_DL_WEB_DIR", "./webui/dist")
	var apiOnly = envBool("MEDIA_DL_API_ONLY", false)
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "启动 HTTP API 和 webui 服务（可选仅启动 API）",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			server, err := apihttp.New(context.Background(), apihttp.Config{
				Port:        port,
				Proxy:       flagProxy,
				Cookie:      flagCookie,
				CookiesFile: flagCookies,
				WebDir:      webDir,
				APIOnly:     apiOnly,
			})
			if err != nil {
				return err
			}
			return server.Run(context.Background())
		},
	}
	serverCmd.Flags().UintVarP(&port, "port", "P", envUint("MEDIA_DL_PORT", 8080), "HTTP 服务端口")
	serverCmd.Flags().StringVar(&webDir, "web-dir", webDir, "webui 构建目录")
	serverCmd.Flags().BoolVar(&apiOnly, "api-only", apiOnly, "仅启动 HTTP API，不注册 WebUI 和验证码相关接口")
	return serverCmd
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envUint(name string, fallback uint) uint {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	var parsed uint
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed == 0 {
		return fallback
	}
	return parsed
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func newVideoCommand() *cobra.Command {
	videoCmd := &cobra.Command{
		Use:     "video",
		Aliases: []string{"v"},
		Short:   "视频解析和下载",
		Args:    cobra.NoArgs,
	}

	infoCmd := &cobra.Command{
		Use:   "info <platform> <url>",
		Short: "只解析视频信息，输出统一 JSON（不下载）",
		Args:  cobra.ExactArgs(2),
		RunE:  runVideoInfo,
	}

	dlCmd := &cobra.Command{
		Use:     "download <platform> <url>",
		Aliases: []string{"dl"},
		Short:   "解析并下载视频",
		Args:    cobra.ExactArgs(2),
		RunE:    runVideoDownload,
	}
	dlCmd.Flags().StringVarP(&flagOutput, "output", "o", ".", "保存目录")
	dlCmd.Flags().StringVarP(&flagFormat, "format", "f", "mp4", "输出容器格式 (mp4/mkv/...)")
	dlCmd.Flags().StringVarP(&flagName, "name", "n", "", "输出文件名（不含扩展名）")
	dlCmd.Flags().BoolVar(&flagCover, "cover", false, "同时下载封面图")

	videoCmd.AddCommand(infoCmd, dlCmd)
	return videoCmd
}

func newMusicCommand() *cobra.Command {
	musicCmd := &cobra.Command{
		Use:     "music",
		Aliases: []string{"m"},
		Short:   "音乐搜索和下载",
		Args:    cobra.NoArgs,
	}

	musicSearchCmd := &cobra.Command{
		Use:     "search [keyword]",
		Aliases: []string{"s"},
		Short:   "搜索歌曲（默认搜索全部音乐平台）",
		Args:    cobra.MaximumNArgs(1),
		RunE:    runMusicSearch,
	}
	musicSearchCmd.Flags().StringSliceVarP(&flagMusicPlatforms, "platform", "p", nil, "指定音乐平台，可逗号分隔；不传则搜索全部平台")
	musicSearchCmd.Flags().StringVarP(&flagMusicArtist, "artist", "a", "", "歌手名")
	musicSearchCmd.Flags().StringVar(&flagMusicArtist, "singer", "", "歌手名（--artist 的别名）")
	musicSearchCmd.Flags().StringVarP(&flagMusicTitle, "title", "t", "", "歌曲名")
	musicSearchCmd.Flags().IntVarP(&flagMusicLimit, "limit", "l", 10, "每个平台最多返回结果数")

	musicDownloadCmd := &cobra.Command{
		Use:     "download <platform> <url>",
		Aliases: []string{"dl"},
		Short:   "解析并下载歌曲",
		Args:    cobra.ExactArgs(2),
		RunE:    runMusicDownload,
	}
	musicDownloadCmd.Flags().StringVarP(&flagMusicOutput, "output", "o", "./downloads", "保存目录")
	musicDownloadCmd.Flags().StringVarP(&flagMusicName, "name", "n", "", "输出文件名（不含扩展名）")
	musicDownloadCmd.Flags().BoolVar(&flagMusicCover, "cover", false, "同时下载封面图")
	musicDownloadCmd.Flags().BoolVar(&flagMusicLyrics, "lyrics", false, "同时下载歌词文件")

	musicCmd.AddCommand(musicSearchCmd, musicDownloadCmd)
	return musicCmd
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
			"kugou.com", "5sing.kugou.com", "kuwo.cn", "migu.cn",
			"music.163.com", "qqmusic.qq.com", "qishui.com", "jamendo.com", "joox.com",
			"apple.com", "music.apple.com",
			"youtube.com", "youtu.be", "tiktok.com", "kuaishou.com", "kuaishouapp.com", "kwai.com",
			"baidu.com", "x.com", "twitter.com", "douyu.com", "huya.com",
		} {
			cookies = append(cookies, httpx.ParseCookieHeader(flagCookie, domain)...)
		}
	}
	return httpx.New(httpx.Options{
		Proxy:   flagProxy,
		Cookies: cookies,
	})
}

func runVideoInfo(_ *cobra.Command, args []string) error {
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

func runVideoDownload(_ *cobra.Command, args []string) error {
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

func runMusicSearch(_ *cobra.Command, args []string) error {
	if err := music.ConfigureProxy(flagProxy); err != nil {
		return err
	}
	cookie, err := musicCookie()
	if err != nil {
		return err
	}
	keyword := ""
	if len(args) == 1 {
		keyword = args[0]
	}
	resp, searchErr := music.New(cookie).Search(music.SearchOptions{
		Keyword:   keyword,
		Artist:    flagMusicArtist,
		Title:     flagMusicTitle,
		Platforms: flagMusicPlatforms,
		Limit:     flagMusicLimit,
	})
	if resp == nil {
		return searchErr
	}
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	if searchErr != nil {
		return jsonPrintedError{searchErr}
	}
	return nil
}

func runMusicDownload(_ *cobra.Command, args []string) error {
	if err := music.ConfigureProxy(flagProxy); err != nil {
		return err
	}
	cookie, err := musicCookie()
	if err != nil {
		return err
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	platform, err := music.NormalizePlatform(args[0])
	if err != nil {
		return err
	}
	service := music.New(cookie)
	fmt.Fprintln(os.Stderr, "解析音乐链接...")
	song, err := service.Parse(client, platform, args[1])
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "平台: %s\n", song.Source)
	fmt.Fprintf(os.Stderr, "歌曲: %s\n", song.Name)
	if song.Artist != "" {
		fmt.Fprintf(os.Stderr, "歌手: %s\n", song.Artist)
	}

	result, err := service.Download(client, song, music.DownloadOptions{
		OutputDir:      flagMusicOutput,
		Filename:       flagMusicName,
		DownloadCover:  flagMusicCover,
		DownloadLyrics: flagMusicLyrics,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "完成: %s\n", result.AudioPath)
	if result.CoverPath != "" {
		fmt.Fprintf(os.Stderr, "封面: %s\n", result.CoverPath)
	}
	if result.LyricsPath != "" {
		fmt.Fprintf(os.Stderr, "歌词: %s\n", result.LyricsPath)
	}
	return nil
}

func musicCookie() (string, error) {
	parts := make([]string, 0, 2)
	if value := strings.TrimSpace(flagCookie); value != "" {
		parts = append(parts, value)
	}
	if flagCookies != "" {
		cookies, err := downloader.LoadCookiesFile(flagCookies)
		if err != nil {
			return "", fmt.Errorf("读取 cookies 文件失败: %w", err)
		}
		for _, cookie := range cookies {
			if cookie == nil || cookie.Name == "" {
				continue
			}
			parts = append(parts, cookie.Name+"="+cookie.Value)
		}
	}
	return strings.Join(parts, "; "), nil
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
