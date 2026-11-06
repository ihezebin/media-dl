package server

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"
	lm "github.com/hezebin/media-dl/server/middleware"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"

	"github.com/gin-gonic/gin"
	"github.com/hezebin/media-dl/component/cache"
	"github.com/hezebin/media-dl/config"
	"github.com/hezebin/media-dl/internal/music"
	"github.com/hezebin/media-dl/server/router"
	"github.com/ihezebin/olympus/httpserver"
	"github.com/ihezebin/olympus/httpserver/middleware"
)

type appServer interface {
	RegisterRoutes(...httpserver.RegisterRoutes)
	RegisterOpenAPIUI(string, httpserver.OpenAPIUIBuilder) error
	Engine() *gin.Engine
	RunWithNotifySignal(context.Context) error
}

func New(ctx context.Context, conf Config) (*Server, error) {
	if err := conf.NormalizeLogging(); err != nil {
		return nil, err
	}
	if conf.Port == 0 {
		conf.Port = 8081
	}
	if conf.WebDir == "" {
		conf.WebDir = "./webui/dist"
	}
	webDir, err := filepath.Abs(conf.WebDir)
	if err != nil {
		return nil, fmt.Errorf("解析 webui 目录失败: %w", err)
	}
	if err := music.ConfigureProxy(conf.Proxy); err != nil {
		return nil, err
	}
	var captchaStore *cache.BehaviorCaptchaStore
	if !conf.APIOnly {
		captchaStore, err = cache.NewBehaviorCaptchaStore()
		if err != nil {
			return nil, err
		}
	}
	meter := otel.Meter(fmt.Sprintf("%s/metrics", conf.ServiceName))
	metricRequestCounter, err := lm.MetricRequestCounter(meter)
	if err != nil {
		return nil, errors.Wrap(err, "create metric request counter failed")
	}

	server, err := httpserver.NewServer(ctx,
		httpserver.WithPort(conf.Port),
		httpserver.WithServiceName(conf.ServiceName),
		httpserver.WithMetrics(),
		httpserver.WithPprof(),
		httpserver.WithMiddlewares(
			middleware.Recovery(),
			lm.Cors(),
			metricRequestCounter,
		),
		// httpserver.WithLogProcessor(logExporter),
		// httpserver.WithTraceExporter(traceExporter),
		httpserver.WithOpenAPInfo(openapi3.Info{
			Version:     "1.0",
			Description: "Media DL API 文档",
			Contact: &openapi3.Contact{
				Name:  "ihezebin",
				Email: "ihezebin@qq.com",
			},
		}),
		httpserver.WithOpenAPIServer(
			openapi3.Server{
				URL:         fmt.Sprintf("http://localhost:%d", conf.Port),
				Description: "本地开发环境",
			},
			openapi3.Server{
				URL:         "http://media-dl.hezebin.com",
				Description: "线上环境",
			},
		),
	)
	if err != nil {
		return nil, errors.Wrap(err, "new server err")
	}

	routes := router.New(server.Engine(), conf, webDir, captchaStore)
	musicRouter := router.NewMusicRouter(conf, captchaStore)
	result := &Server{app: server, Router: routes, Music: musicRouter}
	server.RegisterRoutes(musicRouter, router.NewVideoRouter(conf, captchaStore), router.NewCaptchaRouter(conf, captchaStore))
	err = server.RegisterOpenAPIUI("/openapi", httpserver.StoplightUI)
	if err != nil {
		return nil, errors.Wrap(err, "register openapi ui err")
	}
	_ = server.RegisterOpenAPIUI("/redoc", httpserver.RedocUI)
	_ = server.RegisterOpenAPIUI("/rapidoc", httpserver.RapidocUI)
	_ = server.RegisterOpenAPIUI("/swagger", httpserver.SwaggerUI)

	routes.RegisterFilesAndWeb()
	return result, nil
}

func (s *Server) Run(ctx context.Context) error {
	return s.app.RunWithNotifySignal(ctx)
}

type Config = config.Config
type Server struct {
	app    appServer
	Router *router.Router
	Music  *router.MusicRouter
}
