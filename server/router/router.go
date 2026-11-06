package router

import (
	"github.com/gin-gonic/gin"
	"github.com/hezebin/media-dl/application/service"
	"github.com/hezebin/media-dl/component/cache"
	"github.com/hezebin/media-dl/config"
)

type Router struct {
	Engine  *gin.Engine
	config  config.Config
	webDir  string
	Service *service.Service
}

func New(engine *gin.Engine, cfg config.Config, webDir string, captcha *cache.BehaviorCaptchaStore) *Router {
	return &Router{Engine: engine, config: cfg, webDir: webDir, Service: service.New(cfg, captcha)}
}
