// Package httpserver 提供 media-dl 的 HTTP API 和 webui 静态文件服务。
package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	musicmodel "github.com/guohuiyuan/music-lib/model"
	olympus "github.com/ihezebin/olympus/httpserver"
	"github.com/ihezebin/olympus/httpserver/middleware"

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

type Config struct {
	Port        uint
	Proxy       string
	Cookie      string
	CookiesFile string
	WebDir      string
	APIOnly     bool
}

type Server struct {
	app          appServer
	config       Config
	webDir       string
	captchaStore *behaviorCaptchaStore
}

type appServer interface {
	RegisterRoutes(...olympus.RegisterRoutes)
	RegisterOpenAPIUI(string, olympus.OpenAPIUIBuilder) error
	Engine() *gin.Engine
	RunWithNotifySignal(context.Context) error
}

func New(ctx context.Context, cfg Config) (*Server, error) {
	if cfg.Port == 0 {
		cfg.Port = 8081
	}
	if cfg.WebDir == "" {
		cfg.WebDir = "./webui/dist"
	}
	webDir, err := filepath.Abs(cfg.WebDir)
	if err != nil {
		return nil, fmt.Errorf("解析 webui 目录失败: %w", err)
	}
	if err := music.ConfigureProxy(cfg.Proxy); err != nil {
		return nil, err
	}
	var captchaStore *behaviorCaptchaStore
	if !cfg.APIOnly {
		captchaStore, err = newBehaviorCaptchaStore()
		if err != nil {
			return nil, err
		}
	}
	app, err := olympus.NewServer(
		ctx,
		olympus.WithPort(cfg.Port),
		olympus.WithServiceName("media-dl"),
		olympus.WithMiddlewares(corsMiddleware(), middleware.Recovery(), middleware.LoggingRequestWithoutHeader(), middleware.LoggingResponseWithoutHeader()),
		olympus.WithHiddenRoutesLog(),
	)
	if err != nil {
		return nil, err
	}

	server := &Server{app: app, config: cfg, webDir: webDir, captchaStore: captchaStore}
	app.RegisterRoutes(server)
	if err := app.RegisterOpenAPIUI("/openapi", olympus.StoplightUI); err != nil {
		return nil, fmt.Errorf("注册 OpenAPI 文档失败: %w", err)
	}
	server.registerFilesAndWeb()
	return server, nil
}

func (s *Server) Run(ctx context.Context) error {
	return s.app.RunWithNotifySignal(ctx)
}

func (s *Server) RegisterRoutes(router olympus.Router) {
	api := router.Group("/api")
	api.GET("/music/platforms", olympus.NewHandler(s.musicPlatforms))
	api.POST("/music/search", olympus.NewHandler(s.musicSearch))
	api.POST("/music/resolve", olympus.NewHandler(s.musicResolve))
	api.POST("/music/lyrics", olympus.NewHandler(s.musicLyrics))
	api.POST("/music/download", olympus.NewHandler(s.musicDownload))
	api.POST("/video/info", olympus.NewHandler(s.videoInfo))
	api.POST("/video/download", olympus.NewHandler(s.videoDownload))
	if s.config.APIOnly {
		return
	}
	api.GET("/captcha", olympus.NewHandler(s.captcha))
	api.POST("/captcha/verify", olympus.NewHandler(s.captchaVerify))
	api.POST("/music/search/verified", olympus.NewHandler(s.musicSearchVerified))
	api.POST("/video/info/verified", olympus.NewHandler(s.videoInfoVerified))
}

type platformsResponse struct {
	Platforms []string `json:"platforms"`
}

func (s *Server) musicPlatforms(_ *gin.Context, _ olympus.EmptyType) (*platformsResponse, error) {
	return &platformsResponse{Platforms: append([]string(nil), music.PlatformNames...)}, nil
}

type musicSearchRequest struct {
	Keyword   string            `json:"keyword"`
	Type      string            `json:"type"`
	Artist    string            `json:"artist"`
	Title     string            `json:"title"`
	Album     string            `json:"album"`
	Platforms []string          `json:"platforms"`
	Limit     int               `json:"limit"`
	Cookies   map[string]string `json:"cookies"`
}

func (s *Server) musicSearch(_ *gin.Context, req musicSearchRequest) (*music.SearchResponse, error) {
	cookie, err := s.musicCookie()
	if err != nil {
		return nil, badRequest(err)
	}
	keyword := strings.TrimSpace(req.Keyword)
	artist := strings.TrimSpace(req.Artist)
	title := strings.TrimSpace(req.Title)
	album := strings.TrimSpace(req.Album)
	switch strings.ToLower(strings.TrimSpace(req.Type)) {
	case "artist":
		artist, keyword = keyword, ""
	case "album":
		album, keyword = keyword, ""
	}
	service := music.New(cookie)
	response, err := service.Search(music.SearchOptions{
		Keyword:   keyword,
		Artist:    artist,
		Title:     title,
		Album:     album,
		Platforms: req.Platforms,
		Limit:     req.Limit,
		Cookies:   req.Cookies,
	})
	if response == nil && err != nil {
		return nil, badRequest(err)
	}
	if err != nil && len(response.Results) == 0 {
		return response, badRequest(err)
	}
	return response, nil
}

func (s *Server) musicSearchVerified(ctx *gin.Context, req musicSearchRequest) (*music.SearchResponse, error) {
	if err := s.requireCaptcha(ctx); err != nil {
		return nil, err
	}
	return s.musicSearch(ctx, req)
}

type musicResolveRequest struct {
	Platform string `json:"platform" form:"platform" openapi:"required"`
	URL      string `json:"url" form:"url" openapi:"required"`
	Cookie   string `json:"cookie"`
}

func (s *Server) musicResolve(_ *gin.Context, req musicResolveRequest) (*musicmodel.Song, error) {
	if strings.TrimSpace(req.Platform) == "" || strings.TrimSpace(req.URL) == "" {
		return nil, badRequest(fmt.Errorf("platform 和 url 不能为空"))
	}
	req.URL = unproxyURL(req.URL)
	client, err := s.client()
	if err != nil {
		return nil, badRequest(err)
	}
	cookie := strings.TrimSpace(req.Cookie)
	if cookie == "" {
		cookie, err = s.musicCookie()
		if err != nil {
			return nil, badRequest(err)
		}
	}
	song, err := music.New(cookie).Parse(client, req.Platform, req.URL)
	if err != nil {
		return nil, badRequest(err)
	}
	return song, nil
}

type musicLyricsRequest struct {
	Song   musicmodel.Song `json:"song" openapi:"required"`
	Cookie string          `json:"cookie"`
}

type musicLyricsResponse struct {
	Lyrics string `json:"lyrics"`
}

func (s *Server) musicLyrics(_ *gin.Context, req musicLyricsRequest) (*musicLyricsResponse, error) {
	if req.Song.Source == "" {
		return nil, badRequest(fmt.Errorf("歌曲来源为空"))
	}
	unproxyMusicSong(&req.Song)
	cookie := strings.TrimSpace(req.Cookie)
	var err error
	if cookie == "" {
		cookie, err = s.musicCookie()
		if err != nil {
			return nil, badRequest(err)
		}
	}
	provider, err := music.New(cookie).Provider(req.Song.Source)
	if err != nil {
		return nil, badRequest(err)
	}
	lyrics, err := provider.GetLyrics(&req.Song)
	if err != nil {
		return nil, badRequest(err)
	}
	return &musicLyricsResponse{Lyrics: lyrics}, nil
}

type musicDownloadRequest struct {
	Action   string          `json:"action" openapi:"required"`
	Song     musicmodel.Song `json:"song" openapi:"required"`
	Filename string          `json:"filename"`
	Cookie   string          `json:"cookie"`
}

// downloadResponse 仅用于生成 OpenAPI 响应模型；成功响应实际是二进制附件。
type downloadResponse struct{}

func (s *Server) musicDownload(c *gin.Context, req musicDownloadRequest) (*downloadResponse, error) {
	if req.Song.Source == "" || req.Song.Name == "" {
		return nil, badRequest(fmt.Errorf("歌曲信息不完整"))
	}
	unproxyMusicSong(&req.Song)
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "audio"
	}
	client, err := s.client()
	if err != nil {
		return nil, badRequest(err)
	}
	cookie := strings.TrimSpace(req.Cookie)
	if cookie == "" {
		cookie, err = s.musicCookie()
		if err != nil {
			return nil, badRequest(err)
		}
	}
	tempDir, err := os.MkdirTemp("", "media-dl-http-download-")
	if err != nil {
		return nil, badRequest(fmt.Errorf("创建临时下载目录失败: %w", err))
	}
	defer os.RemoveAll(tempDir)
	result, err := music.New(cookie).DownloadAsset(client, &req.Song, music.AssetOptions{
		Action:    action,
		OutputDir: tempDir,
		Filename:  req.Filename,
	})
	if err != nil {
		return nil, badRequest(err)
	}
	path := result.AudioPath
	if action == "cover" {
		path = result.CoverPath
	}
	if action == "lyrics" || action == "lyric" {
		path = result.LyricsPath
	}
	if path == "" {
		return nil, badRequest(fmt.Errorf("下载结果为空"))
	}
	c.FileAttachment(path, filepath.Base(path))
	return nil, nil
}

type videoInfoRequest struct {
	Platform string `json:"platform" form:"platform"`
	URL      string `json:"url" form:"url" openapi:"required"`
}

func (s *Server) videoInfo(_ *gin.Context, req videoInfoRequest) (*model.VideoInfo, error) {
	if strings.TrimSpace(req.URL) == "" {
		return nil, badRequest(fmt.Errorf("url 不能为空"))
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		platform = detectVideoPlatform(req.URL)
	}
	if platform == "" {
		return nil, badRequest(fmt.Errorf("无法识别视频平台，请传入 platform"))
	}
	client, err := s.client()
	if err != nil {
		return nil, badRequest(err)
	}
	info, err := extractor.Extract(client, platform, req.URL)
	if err != nil {
		return nil, badRequest(err)
	}
	return info, nil
}

func (s *Server) videoInfoVerified(ctx *gin.Context, req videoInfoRequest) (*model.VideoInfo, error) {
	if err := s.requireCaptcha(ctx); err != nil {
		return nil, err
	}
	return s.videoInfo(ctx, req)
}

type videoDownloadRequest struct {
	Platform string `json:"platform"`
	URL      string `json:"url" openapi:"required"`
	Format   string `json:"format"`
	Name     string `json:"name"`
	Cover    bool   `json:"cover"`
}

func (s *Server) videoDownload(c *gin.Context, req videoDownloadRequest) (*downloadResponse, error) {
	info, err := s.videoInfo(c, videoInfoRequest{Platform: req.Platform, URL: req.URL})
	if err != nil {
		return nil, err
	}
	client, err := s.client()
	if err != nil {
		return nil, badRequest(err)
	}
	tempDir, err := os.MkdirTemp("", "media-dl-http-download-")
	if err != nil {
		return nil, badRequest(fmt.Errorf("创建临时下载目录失败: %w", err))
	}
	defer os.RemoveAll(tempDir)
	result, err := downloader.Download(client, info, downloader.Options{
		OutputDir: tempDir,
		Filename:  req.Name,
		Format:    req.Format,
		Quiet:     true,
	})
	if err != nil {
		return nil, badRequest(err)
	}
	if result.VideoPath == "" {
		return nil, badRequest(fmt.Errorf("下载结果为空"))
	}
	c.FileAttachment(result.VideoPath, filepath.Base(result.VideoPath))
	return nil, nil
}

func (s *Server) client() (*httpx.Client, error) {
	var cookies []*http.Cookie
	if s.config.CookiesFile != "" {
		loaded, err := downloader.LoadCookiesFile(s.config.CookiesFile)
		if err != nil {
			return nil, fmt.Errorf("读取 cookies 文件失败: %w", err)
		}
		cookies = append(cookies, loaded...)
	}
	if raw := strings.TrimSpace(s.config.Cookie); raw != "" {
		for _, domain := range []string{"douyin.com", "bilibili.com", "xiaohongshu.com", "weibo.com", "weibo.cn", "youku.com", "tudou.com", "iqiyi.com", "iq.com", "ixigua.com", "toutiao.com", "qq.com", "kugou.com", "5sing.kugou.com", "kuwo.cn", "migu.cn", "music.163.com", "qqmusic.qq.com", "qishui.com", "jamendo.com", "joox.com", "apple.com", "music.apple.com", "youtube.com", "youtu.be", "tiktok.com", "kuaishou.com", "kuaishouapp.com", "kwai.com", "baidu.com", "x.com", "twitter.com", "douyu.com", "huya.com"} {
			cookies = append(cookies, httpx.ParseCookieHeader(raw, domain)...)
		}
	}
	return httpx.New(httpx.Options{Proxy: s.config.Proxy, Cookies: cookies})
}

func (s *Server) musicCookie() (string, error) {
	parts := make([]string, 0, 2)
	if raw := strings.TrimSpace(s.config.Cookie); raw != "" {
		parts = append(parts, raw)
	}
	if s.config.CookiesFile != "" {
		cookies, err := downloader.LoadCookiesFile(s.config.CookiesFile)
		if err != nil {
			return "", fmt.Errorf("读取 cookies 文件失败: %w", err)
		}
		for _, cookie := range cookies {
			if cookie != nil && cookie.Name != "" {
				parts = append(parts, cookie.Name+"="+cookie.Value)
			}
		}
	}
	return strings.Join(parts, "; "), nil
}

func (s *Server) registerFilesAndWeb() {
	engine := s.app.Engine()
	engine.GET(proxyPath, s.proxy)
	if s.config.APIOnly {
		return
	}
	engine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "route not found"})
			return
		}
		rel := strings.TrimPrefix(c.Request.URL.Path, "/")
		candidate := filepath.Join(s.webDir, filepath.Clean(rel))
		if within(s.webDir, candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				c.File(candidate)
				return
			}
		}
		index := filepath.Join(s.webDir, "index.html")
		if _, err := os.Stat(index); err == nil {
			c.File(index)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "webui dist not found"})
	})
}

func detectVideoPlatform(rawURL string) string {
	for _, item := range extractor.All() {
		if item.Match(rawURL) {
			return item.Name()
		}
	}
	return ""
}

func within(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func badRequest(err error) error {
	return olympus.NewError(olympus.CodeBadRequest, err.Error())
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
