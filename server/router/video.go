package router

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/hezebin/media-dl/application/dto"
	"github.com/hezebin/media-dl/internal/video/model"
)

func (s *Router) videoInfo(c *gin.Context, req dto.VideoInfoRequest) (*model.VideoInfo, error) {
	return s.Service.VideoInfo(c.Request.Context(), req)
}

func (s *Router) videoDownload(c *gin.Context, req dto.VideoDownloadRequest) (*dto.DownloadResponse, error) {
	return s.Service.VideoDownload(c.Request.Context(), req, func(path string) error { c.FileAttachment(path, filepath.Base(path)); return nil })
}
