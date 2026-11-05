package dto

import (
	musicmodel "github.com/guohuiyuan/music-lib/model"
)

type PlatformsResponse struct {
	Platforms []string `json:"platforms"`
}

type MusicSearchRequest struct {
	Keyword     string            `json:"keyword"`
	Type        string            `json:"type"`
	Artist      string            `json:"artist"`
	Title       string            `json:"title"`
	Album       string            `json:"album"`
	Platforms   []string          `json:"platforms"`
	Limit       int               `json:"limit"`
	Cookies     map[string]string `json:"cookies"`
	CookieFiles map[string]string `json:"cookie_files"`
}

type MusicResolveRequest struct {
	Platform   string `json:"platform" form:"platform" openapi:"required"`
	URL        string `json:"url" form:"url" openapi:"required"`
	Cookie     string `json:"cookie"`
	CookieFile string `json:"cookie_file"`
}

type MusicLyricsRequest struct {
	Song       musicmodel.Song `json:"song" openapi:"required"`
	Cookie     string          `json:"cookie"`
	CookieFile string          `json:"cookie_file"`
}

type MusicLyricsResponse struct {
	Lyrics string `json:"lyrics"`
}

type MusicDownloadRequest struct {
	Action     string          `json:"action" openapi:"required"`
	Song       musicmodel.Song `json:"song" openapi:"required"`
	Filename   string          `json:"filename"`
	Cookie     string          `json:"cookie"`
	CookieFile string          `json:"cookie_file"`
}
