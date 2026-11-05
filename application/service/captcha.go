package service

import (
	"fmt"

	"github.com/hezebin/media-dl/component/cache"
	olympus "github.com/ihezebin/olympus/httpserver"
)

func (s *Service) CreateCaptcha() (*cache.BehaviorCaptchaResponse, error) {
	return s.Captcha.Generate()
}
func (s *Service) VerifyCaptcha(req cache.BehaviorCaptchaVerifyRequest) (*cache.BehaviorCaptchaVerifyResponse, error) {
	token, expires, ok := s.Captcha.Verify(req)
	if !ok {
		return nil, BadRequest(fmt.Errorf("验证码校验失败，请重新操作"))
	}
	return &cache.BehaviorCaptchaVerifyResponse{Verified: true, Token: token, ExpiresAt: expires}, nil
}
func (s *Service) RequireCaptcha(token string) error {
	if s.Captcha != nil && s.Captcha.ValidToken(token) {
		return nil
	}
	return olympus.NewError(olympus.CodeUnauthorized, "验证码已失效，请先完成验证")
}
