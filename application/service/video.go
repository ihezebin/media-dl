package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hezebin/media-dl/application/dto"
	"github.com/hezebin/media-dl/component/remote"
	"github.com/hezebin/media-dl/internal/video/downloader"
	"github.com/hezebin/media-dl/internal/video/extractor"
	_ "github.com/hezebin/media-dl/internal/video/extractor/bilibili"
	_ "github.com/hezebin/media-dl/internal/video/extractor/douyin"
	_ "github.com/hezebin/media-dl/internal/video/extractor/iqiyi"
	_ "github.com/hezebin/media-dl/internal/video/extractor/tencent"
	_ "github.com/hezebin/media-dl/internal/video/extractor/web"
	_ "github.com/hezebin/media-dl/internal/video/extractor/weibo"
	_ "github.com/hezebin/media-dl/internal/video/extractor/xiaohongshu"
	_ "github.com/hezebin/media-dl/internal/video/extractor/xigua"
	_ "github.com/hezebin/media-dl/internal/video/extractor/youku"
	"github.com/hezebin/media-dl/internal/video/model"
)

func (s *Service) VideoInfo(ctx context.Context, req dto.VideoInfoRequest) (*model.VideoInfo, error) {
	if strings.TrimSpace(req.URL) == "" {
		return nil, BadRequest(fmt.Errorf("url 不能为空"))
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		platform = remote.DetectVideoPlatform(req.URL)
	}
	if platform == "" {
		return nil, BadRequest(fmt.Errorf("无法识别视频平台，请传入 platform"))
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	client, err := s.ClientWithContext(ctx)
	if err != nil {
		return nil, BadRequest(err)
	}
	remote.ApplyCookieHeader(client, req.Cookie)
	remote.ApplyCookieFile(client, req.CookieFile)
	info, err := extractor.Extract(client, platform, req.URL)
	if ctx.Err() != nil {
		return nil, BadRequest(fmt.Errorf("视频解析超时或已取消，请检查网络和代理后重试"))
	}
	if err != nil {
		return nil, BadRequest(err)
	}
	return info, nil
}

func (s *Service) VideoDownload(ctx context.Context, req dto.VideoDownloadRequest, deliver func(string) error) (*dto.DownloadResponse, error) {
	info, err := s.VideoInfo(ctx, dto.VideoInfoRequest{Platform: req.Platform, URL: req.URL, Cookie: req.Cookie, CookieFile: req.CookieFile})
	if err != nil {
		return nil, err
	}
	client, err := s.client()
	if err != nil {
		return nil, BadRequest(err)
	}
	remote.ApplyCookieHeader(client, req.Cookie)
	remote.ApplyCookieFile(client, req.CookieFile)
	tempDir, err := os.MkdirTemp("", "media-dl-http-download-")
	if err != nil {
		return nil, BadRequest(fmt.Errorf("创建临时下载目录失败: %w", err))
	}
	defer os.RemoveAll(tempDir)
	result, err := downloader.Download(client, info, downloader.Options{
		OutputDir: tempDir,
		Filename:  req.Name,
		Format:    req.Format,
		Quiet:     true,
	})
	if err != nil {
		return nil, BadRequest(err)
	}
	if result.VideoPath == "" {
		return nil, BadRequest(fmt.Errorf("下载结果为空"))
	}
	if err := deliver(result.VideoPath); err != nil {
		return nil, err
	}
	return nil, nil
}
