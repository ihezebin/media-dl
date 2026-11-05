package router

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
	musicmodel "github.com/guohuiyuan/music-lib/model"
	"github.com/hezebin/media-dl/application/dto"
	"github.com/hezebin/media-dl/internal/music"
	olympus "github.com/ihezebin/olympus/httpserver"
)

func (s *Router) musicPlatforms(c *gin.Context, _ olympus.EmptyType) (*dto.PlatformsResponse, error) {
	return s.Service.MusicPlatforms(c.Request.Context())
}

func (s *Router) musicSearch(c *gin.Context, req dto.MusicSearchRequest) (*music.SearchResponse, error) {
	return s.Service.MusicSearch(c.Request.Context(), req)
}

func (s *Router) musicResolve(c *gin.Context, req dto.MusicResolveRequest) (*musicmodel.Song, error) {
	return s.Service.MusicResolve(c.Request.Context(), req)
}

func (s *Router) musicLyrics(c *gin.Context, req dto.MusicLyricsRequest) (*dto.MusicLyricsResponse, error) {
	return s.Service.MusicLyrics(c.Request.Context(), req)
}

func (s *Router) musicDownload(c *gin.Context, req dto.MusicDownloadRequest) (*dto.DownloadResponse, error) {
	return s.Service.MusicDownload(c.Request.Context(), req, func(path string) error { c.FileAttachment(path, filepath.Base(path)); return nil })
}
