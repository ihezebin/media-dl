package service

import (
	"context"
	"fmt"
	"os"
	"strings"

	musicmodel "github.com/guohuiyuan/music-lib/model"
	"github.com/hezebin/media-dl/application/dto"
	"github.com/hezebin/media-dl/component/remote"
	"github.com/hezebin/media-dl/internal/music"
)

func (s *Service) MusicPlatforms(ctx context.Context) (*dto.PlatformsResponse, error) {
	return &dto.PlatformsResponse{Platforms: append([]string(nil), music.PlatformNames...)}, nil
}

func (s *Service) MusicSearch(ctx context.Context, req dto.MusicSearchRequest) (*music.SearchResponse, error) {
	cookie, err := s.musicCookie()
	if err != nil {
		return nil, BadRequest(err)
	}
	keyword := strings.TrimSpace(req.Keyword)
	artist := strings.TrimSpace(req.Artist)
	title := strings.TrimSpace(req.Title)
	album := strings.TrimSpace(req.Album)
	switch strings.ToLower(strings.TrimSpace(req.Type)) {
	case "artist":
		artist, keyword = keyword, ""
	case "album":
		album, keyword = keyword, ""
	}
	cookies := req.Cookies
	if len(req.CookieFiles) > 0 {
		if cookies == nil {
			cookies = make(map[string]string)
		}
		for platform, raw := range req.CookieFiles {
			value, fileErr := remote.CookieHeaderFromTextForPlatform(raw, platform)
			if fileErr != nil {
				return nil, BadRequest(fileErr)
			}
			if value != "" {
				cookies[platform] = value
			}
		}
	}
	service := music.New(cookie)
	response, err := service.Search(music.SearchOptions{
		Keyword:   keyword,
		Artist:    artist,
		Title:     title,
		Album:     album,
		Platforms: req.Platforms,
		Limit:     req.Limit,
		Cookies:   cookies,
	})
	if response == nil && err != nil {
		return nil, BadRequest(err)
	}
	if err != nil && len(response.Results) == 0 {
		return response, BadRequest(err)
	}
	return response, nil
}

func (s *Service) MusicResolve(ctx context.Context, req dto.MusicResolveRequest) (*musicmodel.Song, error) {
	if strings.TrimSpace(req.Platform) == "" || strings.TrimSpace(req.URL) == "" {
		return nil, BadRequest(fmt.Errorf("platform 和 url 不能为空"))
	}
	req.URL = remote.UnproxyURL(req.URL)
	client, err := s.client()
	if err != nil {
		return nil, BadRequest(err)
	}
	cookie, err := remote.RequestMusicCookie(req.Cookie, req.CookieFile)
	if err != nil {
		return nil, BadRequest(err)
	}
	if cookie == "" {
		cookie, err = s.musicCookie()
		if err != nil {
			return nil, BadRequest(err)
		}
	}
	remote.ApplyCookieFile(client, req.CookieFile)
	song, err := music.New(cookie).Parse(client, req.Platform, req.URL)
	if err != nil {
		return nil, BadRequest(err)
	}
	return song, nil
}

func (s *Service) MusicLyrics(ctx context.Context, req dto.MusicLyricsRequest) (*dto.MusicLyricsResponse, error) {
	if req.Song.Source == "" {
		return nil, BadRequest(fmt.Errorf("歌曲来源为空"))
	}
	remote.UnproxyMusicSong(&req.Song)
	cookie, err := remote.RequestMusicCookie(req.Cookie, req.CookieFile)
	if err != nil {
		return nil, BadRequest(err)
	}
	if cookie == "" {
		cookie, err = s.musicCookie()
		if err != nil {
			return nil, BadRequest(err)
		}
	}
	provider, err := music.New(cookie).Provider(req.Song.Source)
	if err != nil {
		return nil, BadRequest(err)
	}
	lyrics, err := provider.GetLyrics(&req.Song)
	if err != nil {
		return nil, BadRequest(err)
	}
	return &dto.MusicLyricsResponse{Lyrics: lyrics}, nil
}

func (s *Service) MusicDownload(ctx context.Context, req dto.MusicDownloadRequest, deliver func(string) error) (*dto.DownloadResponse, error) {
	if req.Song.Source == "" || req.Song.Name == "" {
		return nil, BadRequest(fmt.Errorf("歌曲信息不完整"))
	}
	remote.UnproxyMusicSong(&req.Song)
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "audio"
	}
	client, err := s.client()
	if err != nil {
		return nil, BadRequest(err)
	}
	cookie, err := remote.RequestMusicCookie(req.Cookie, req.CookieFile)
	if err != nil {
		return nil, BadRequest(err)
	}
	if cookie == "" {
		cookie, err = s.musicCookie()
		if err != nil {
			return nil, BadRequest(err)
		}
	}
	remote.ApplyCookieFile(client, req.CookieFile)
	tempDir, err := os.MkdirTemp("", "media-dl-http-download-")
	if err != nil {
		return nil, BadRequest(fmt.Errorf("创建临时下载目录失败: %w", err))
	}
	defer os.RemoveAll(tempDir)
	result, err := music.New(cookie).DownloadAsset(client, &req.Song, music.AssetOptions{
		Action:    action,
		OutputDir: tempDir,
		Filename:  req.Filename,
	})
	if err != nil {
		return nil, BadRequest(err)
	}
	path := result.AudioPath
	if action == "cover" {
		path = result.CoverPath
	}
	if action == "lyrics" || action == "lyric" {
		path = result.LyricsPath
	}
	if path == "" {
		return nil, BadRequest(fmt.Errorf("下载结果为空"))
	}
	if err := deliver(path); err != nil {
		return nil, err
	}
	return nil, nil
}
