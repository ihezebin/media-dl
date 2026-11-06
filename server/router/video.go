package router

import (
	"github.com/hezebin/media-dl/application/service"
	"github.com/hezebin/media-dl/component/cache"
	"github.com/hezebin/media-dl/config"
	olympus "github.com/ihezebin/olympus/httpserver"
	"github.com/ihezebin/olympus/httpserver/middleware"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/hezebin/media-dl/application/dto"
	"github.com/hezebin/media-dl/internal/video/model"
)

func (s *VideoRouter) videoInfo(c *gin.Context, req dto.VideoInfoRequest) (*model.VideoInfo, error) {
	return s.Service.VideoInfo(c.Request.Context(), req)
}

func (s *VideoRouter) videoDownload(c *gin.Context, req dto.VideoDownloadRequest) (*dto.DownloadResponse, error) {
	return s.Service.VideoDownload(c.Request.Context(), req, func(path string) error { c.FileAttachment(path, filepath.Base(path)); return nil })
}

type VideoRouter struct {
	Service *service.Service
	config  config.Config
}

func NewVideoRouter(cfg config.Config, captcha *cache.BehaviorCaptchaStore) *VideoRouter {
	return &VideoRouter{Service: service.New(cfg, captcha), config: cfg}
}
func (s *VideoRouter) RegisterRoutes(router olympus.Router) {
	group := router.Group("/api/video")
	group.Use(middleware.LoggingRequestWithoutHeader(), middleware.LoggingResponseWithoutHeader())
	group.POST("/info", olympus.NewHandler(s.videoInfo))
	group.POST("/download", olympus.NewHandler(s.videoDownload))
	if !s.config.APIOnly {
		group.POST("/info/verified", olympus.NewHandler(s.videoInfoVerified))
	}
}
func (s *VideoRouter) videoInfoVerified(c *gin.Context, req dto.VideoInfoRequest) (*model.VideoInfo, error) {
	if err := s.Service.RequireCaptcha(c.GetHeader("X-Captcha-Token")); err != nil {
		return nil, err
	}
	return s.videoInfo(c, req)
}
