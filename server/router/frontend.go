package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ihezebin/olympus/httpserver/middleware"
)

func (s *Router) RegisterFilesAndWeb() {
	engine := s.Engine
	group := engine.Group("/api")
	group.Use(middleware.LoggingRequestWithoutHeader(), middleware.LoggingResponseWithoutHeader())
	group.GET("/proxy", s.proxy)
	if s.config.APIOnly {
		return
	}
	engine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "route not found"})
			return
		}
		rel := strings.TrimPrefix(c.Request.URL.Path, "/")
		candidate := filepath.Join(s.webDir, filepath.Clean(rel))
		if within(s.webDir, candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				c.File(candidate)
				return
			}
		}
		index := filepath.Join(s.webDir, "index.html")
		if _, err := os.Stat(index); err == nil {
			c.File(index)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "webui dist not found"})
	})
}

func within(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}
