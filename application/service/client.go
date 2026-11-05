package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/hezebin/media-dl/component/remote"
	"github.com/hezebin/media-dl/internal/httpx"
	"github.com/hezebin/media-dl/internal/video/downloader"
)

func (s *Service) client() (*httpx.Client, error) {
	return s.ClientWithContext(context.Background())
}

func (s *Service) ClientWithContext(ctx context.Context) (*httpx.Client, error) {
	return remote.NewMediaClient(ctx, s.config)
}

func (s *Service) musicCookie() (string, error) {
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
