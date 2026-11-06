package router

import (
	"github.com/hezebin/media-dl/application/service"
	"github.com/hezebin/media-dl/component/cache"
	"github.com/hezebin/media-dl/config"
	"github.com/ihezebin/olympus/httpserver/middleware"
	"path/filepath"

	"github.com/gin-gonic/gin"
	musicmodel "github.com/guohuiyuan/music-lib/model"
	"github.com/hezebin/media-dl/application/dto"
	"github.com/hezebin/media-dl/internal/music"
	olympus "github.com/ihezebin/olympus/httpserver"
)

func (s *MusicRouter) musicPlatforms(c *gin.Context, _ olympus.EmptyType) (*dto.PlatformsResponse, error) {
	return s.Service.MusicPlatforms(c.Request.Context())
}

func (s *MusicRouter) musicSearch(c *gin.Context, req dto.MusicSearchRequest) (*music.SearchResponse, error) {
	return s.Service.MusicSearch(c.Request.Context(), req)
}

func (s *MusicRouter) musicResolve(c *gin.Context, req dto.MusicResolveRequest) (*musicmodel.Song, error) {
	return s.Service.MusicResolve(c.Request.Context(), req)
}

func (s *MusicRouter) musicLyrics(c *gin.Context, req dto.MusicLyricsRequest) (*dto.MusicLyricsResponse, error) {
	return s.Service.MusicLyrics(c.Request.Context(), req)
}

func (s *MusicRouter) musicDownload(c *gin.Context, req dto.MusicDownloadRequest) (*dto.DownloadResponse, error) {
	return s.Service.MusicDownload(c.Request.Context(), req, func(path string) error { c.FileAttachment(path, filepath.Base(path)); return nil })
}

type MusicRouter struct {
	Service *service.Service
	config  config.Config
}

func NewMusicRouter(cfg config.Config, captcha *cache.BehaviorCaptchaStore) *MusicRouter {
	return &MusicRouter{Service: service.New(cfg, captcha), config: cfg}
}
func (s *MusicRouter) RegisterRoutes(router olympus.Router) {
	group := router.Group("/api/music")
	group.Use(middleware.LoggingRequestWithoutHeader(), middleware.LoggingResponseWithoutHeader())
	group.GET("/platforms", olympus.NewHandler(s.musicPlatforms))
	group.POST("/search", olympus.NewHandler(s.musicSearch))
	group.POST("/resolve", olympus.NewHandler(s.musicResolve))
	group.POST("/lyrics", olympus.NewHandler(s.musicLyrics))
	group.POST("/download", olympus.NewHandler(s.musicDownload))
	if !s.config.APIOnly {
		group.POST("/search/verified", olympus.NewHandler(s.musicSearchVerified))
	}
}
func (s *MusicRouter) musicSearchVerified(c *gin.Context, req dto.MusicSearchRequest) (*music.SearchResponse, error) {
	if err := s.Service.RequireCaptcha(c.GetHeader("X-Captcha-Token")); err != nil {
		return nil, err
	}
	return s.musicSearch(c, req)
}
