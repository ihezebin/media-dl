package service

import (
	"time"

	"github.com/hezebin/media-dl/component/cache"
	"github.com/hezebin/media-dl/config"
)

type Captcha interface {
	Generate() (*cache.BehaviorCaptchaResponse, error)
	Verify(cache.BehaviorCaptchaVerifyRequest) (string, time.Time, bool)
	ValidToken(string) bool
}
type Service struct {
	config  config.Config
	Captcha Captcha
}

func New(cfg config.Config, captcha Captcha) *Service { return &Service{config: cfg, Captcha: captcha} }
