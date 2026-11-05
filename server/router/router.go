package router

import (
	"github.com/gin-gonic/gin"
	"github.com/hezebin/media-dl/application/service"
	"github.com/hezebin/media-dl/config"
	olympus "github.com/ihezebin/olympus/httpserver"
)

func (s *Router) RegisterRoutes(router olympus.Router) {
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

type Router struct {
	Engine  *gin.Engine
	config  config.Config
	webDir  string
	Service *service.Service
}

func New(engine *gin.Engine, cfg config.Config, webDir string, app *service.Service) *Router {
	return &Router{Engine: engine, config: cfg, webDir: webDir, Service: app}
}
