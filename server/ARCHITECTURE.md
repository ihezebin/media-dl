# Media DL HTTP 服务组织

遵循 [Doorway 分层标准](../../doorway/server/ARCHITECTURE.md)。本项目兼有 CLI 和 HTTP 两个入口，CLI 继续复用 `internal/music`、`internal/video` 的解析和下载库。

- 根目录 `config` 维护服务参数。
- `component/cache` 初始化并管理人机验证状态；`component/remote` 构造客户端、读取 Cookie 和处理代理 URL。
- `application/dto` 按音乐、视频维护请求/响应模型，保留 Cookie 日志脱敏。
- `application/service` 负责音乐搜索、解析、下载、视频解析下载和人机验证业务，不引入 Gin Context。
- `server/server.go` 组装组件、应用服务和 HTTP 主机；`server/router` 按音乐、视频、人机验证、代理和前端托管处理 HTTP。
- 下载服务同步回调交付本地文件，交付结束后再清理临时目录，避免返回一个随后失效的临时路径。

本服务不持久化业务数据，不创建空数据库仓储。旧 `httpserver` 目录已移除，CLI 导入 `server`，没有保留两套 HTTP 实现。

在项目根目录执行 `go test -race ./...`、`go vet ./...`、`go build ./...`。第三方平台在线解析与系统代理需要单独验证。
