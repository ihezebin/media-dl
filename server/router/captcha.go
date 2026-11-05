package router

import (
	"github.com/hezebin/media-dl/application/dto"

	"github.com/gin-gonic/gin"
	"github.com/hezebin/media-dl/component/cache"
	"github.com/hezebin/media-dl/internal/music"
	"github.com/hezebin/media-dl/internal/video/model"
	olympus "github.com/ihezebin/olympus/httpserver"
)

func (s *Router) musicSearchVerified(ctx *gin.Context, req dto.MusicSearchRequest) (*music.SearchResponse, error) {
	if err := s.requireCaptcha(ctx); err != nil {
		return nil, err
	}
	return s.musicSearch(ctx, req)
}

func (s *Router) videoInfoVerified(ctx *gin.Context, req dto.VideoInfoRequest) (*model.VideoInfo, error) {
	if err := s.requireCaptcha(ctx); err != nil {
		return nil, err
	}
	return s.videoInfo(ctx, req)
}

func (s *Router) captcha(c *gin.Context, _ olympus.EmptyType) (*cache.BehaviorCaptchaResponse, error) {
	return s.Service.CreateCaptcha()
}

func (s *Router) captchaVerify(_ *gin.Context, req cache.BehaviorCaptchaVerifyRequest) (*cache.BehaviorCaptchaVerifyResponse, error) {
	return s.Service.VerifyCaptcha(req)
}

func (s *Router) requireCaptcha(c *gin.Context) error {
	return s.Service.RequireCaptcha(c.GetHeader("X-Captcha-Token"))
}
