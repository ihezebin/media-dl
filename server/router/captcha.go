package router

import (
	"github.com/hezebin/media-dl/application/service"
	"github.com/hezebin/media-dl/config"
	"github.com/ihezebin/olympus/httpserver/middleware"

	"github.com/gin-gonic/gin"
	"github.com/hezebin/media-dl/component/cache"
	olympus "github.com/ihezebin/olympus/httpserver"
)

func (s *CaptchaRouter) captcha(c *gin.Context, _ olympus.EmptyType) (*cache.BehaviorCaptchaResponse, error) {
	return s.Service.CreateCaptcha()
}

func (s *CaptchaRouter) captchaVerify(_ *gin.Context, req cache.BehaviorCaptchaVerifyRequest) (*cache.BehaviorCaptchaVerifyResponse, error) {
	return s.Service.VerifyCaptcha(req)
}

type CaptchaRouter struct {
	Service *service.Service
	config  config.Config
}

func NewCaptchaRouter(cfg config.Config, captcha *cache.BehaviorCaptchaStore) *CaptchaRouter {
	return &CaptchaRouter{Service: service.New(cfg, captcha), config: cfg}
}
func (s *CaptchaRouter) RegisterRoutes(router olympus.Router) {
	if s.config.APIOnly {
		return
	}
	group := router.Group("/api/captcha")
	group.Use(middleware.LoggingRequestWithoutHeader(), middleware.LoggingResponseWithoutHeader())
	group.GET("", olympus.NewHandler(s.captcha))
	group.POST("/verify", olympus.NewHandler(s.captchaVerify))
}
