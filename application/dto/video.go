package dto

import (
	"encoding/json"
	"strings"
)

type DownloadResponse struct{}

type VideoInfoRequest struct {
	Platform   string `json:"platform" form:"platform"`
	URL        string `json:"url" form:"url" openapi:"required"`
	Cookie     string `json:"cookie,omitempty" form:"cookie" header:"-"`
	CookieFile string `json:"cookie_file,omitempty" header:"-"`
}

func (r VideoInfoRequest) MarshalJSON() ([]byte, error) {
	type safeVideoInfoRequest struct {
		Platform string `json:"platform,omitempty"`
		URL      string `json:"url,omitempty"`
		Cookie   string `json:"cookie,omitempty"`
	}
	cookie := ""
	if strings.TrimSpace(r.Cookie) != "" {
		cookie = "[redacted]"
	}
	return json.Marshal(safeVideoInfoRequest{Platform: r.Platform, URL: r.URL, Cookie: cookie})
}

type VideoDownloadRequest struct {
	Platform   string `json:"platform"`
	URL        string `json:"url" openapi:"required"`
	Format     string `json:"format"`
	Name       string `json:"name"`
	Cover      bool   `json:"cover"`
	Cookie     string `json:"cookie,omitempty" header:"-"`
	CookieFile string `json:"cookie_file,omitempty" header:"-"`
}

func (r VideoDownloadRequest) MarshalJSON() ([]byte, error) {
	type safeVideoDownloadRequest struct {
		Platform string `json:"platform,omitempty"`
		URL      string `json:"url,omitempty"`
		Format   string `json:"format,omitempty"`
		Name     string `json:"name,omitempty"`
		Cover    bool   `json:"cover,omitempty"`
		Cookie   string `json:"cookie,omitempty"`
	}
	cookie := ""
	if strings.TrimSpace(r.Cookie) != "" {
		cookie = "[redacted]"
	}
	return json.Marshal(safeVideoDownloadRequest{
		Platform: r.Platform,
		URL:      r.URL,
		Format:   r.Format,
		Name:     r.Name,
		Cover:    r.Cover,
		Cookie:   cookie,
	})
}
