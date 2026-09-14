package httpserver

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	musicmodel "github.com/guohuiyuan/music-lib/model"
)

const proxyPath = "/api/proxy"
const legacyProxyPath = "/proxy"

func unproxyURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	target, err := url.Parse(raw)
	if err != nil || (target.Path != proxyPath && target.Path != legacyProxyPath) {
		return raw
	}
	if original := strings.TrimSpace(target.Query().Get("url")); original != "" {
		return original
	}
	return raw
}

func isHTTPURL(target *url.URL) bool {
	if target == nil || target.Host == "" || target.User != nil {
		return false
	}
	scheme := strings.ToLower(target.Scheme)
	return scheme == "http" || scheme == "https"
}

func parseProxyTarget(raw string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !isHTTPURL(target) {
		return nil, fmt.Errorf("proxy url must be an absolute HTTP(S) URL")
	}
	// Do not turn the endpoint into a direct route to local or link-local
	// addresses. Hostnames are allowed because music CDNs commonly use many
	// rotating domains; the configured upstream proxy still applies.
	if ip := net.ParseIP(target.Hostname()); ip != nil && isPrivateProxyIP(ip) {
		return nil, fmt.Errorf("proxy url points to a private address")
	}
	return target, nil
}

func isPrivateProxyIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func (s *Server) proxy(c *gin.Context) {
	target, err := parseProxyTarget(c.Query("url"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": err.Error()})
		return
	}

	client, err := s.client()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": http.StatusBadGateway, "message": err.Error()})
		return
	}
	headers := make(map[string]string)
	for _, name := range []string{"Accept", "Range", "If-Range", "User-Agent"} {
		if value := c.GetHeader(name); value != "" {
			headers[name] = value
		}
	}
	for name, value := range defaultMusicProxyHeaders(target) {
		if headers[name] == "" {
			headers[name] = value
		}
	}
	for name, queryName := range map[string]string{
		"Referer":    "referer",
		"Origin":     "origin",
		"User-Agent": "user_agent",
	} {
		if value := c.Query(queryName); isProxyHeaderValue(name, value) {
			headers[name] = value
		}
	}
	resp, err := client.Get(target.String(), headers)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": http.StatusBadGateway, "message": fmt.Sprintf("proxy request failed: %v", err)})
		return
	}
	defer resp.Body.Close()

	for _, name := range []string{
		"Accept-Ranges",
		"Cache-Control",
		"Content-Disposition",
		"Content-Range",
		"ETag",
		"Last-Modified",
	} {
		if value := resp.Header.Get(name); value != "" {
			c.Header(name, value)
		}
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	c.Header("Access-Control-Allow-Origin", "*")
	c.DataFromReader(resp.StatusCode, resp.ContentLength, contentType, resp.Body, nil)
}

func defaultMusicProxyHeaders(target *url.URL) map[string]string {
	host := strings.ToLower(target.Hostname())
	if strings.HasSuffix(host, "music.126.net") {
		return map[string]string{
			"Referer": "http://music.163.com/",
			"Origin":  "http://music.163.com",
		}
	}
	if strings.HasSuffix(host, "kugou.com") {
		return map[string]string{"Referer": "https://www.kugou.com/"}
	}
	if strings.HasSuffix(host, "qq.com") {
		return map[string]string{"Referer": "https://y.qq.com/"}
	}
	if strings.HasSuffix(host, "kuwo.cn") {
		return map[string]string{"Referer": "https://www.kuwo.cn/"}
	}
	return nil
}

func unproxyMusicSong(song *musicmodel.Song) {
	transformMusicSong(song, unproxyURL)
}

func transformMusicSong(song *musicmodel.Song, transform func(string) string) {
	if song == nil {
		return
	}
	song.URL = transform(song.URL)
	song.Cover = transform(song.Cover)
	song.Link = transform(song.Link)
	for key, value := range song.Extra {
		song.Extra[key] = transform(value)
	}
}

func isProxyHeaderValue(name, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return false
	}
	if name == "Referer" || name == "Origin" {
		target, err := url.Parse(value)
		return err == nil && isHTTPURL(target)
	}
	return true
}
