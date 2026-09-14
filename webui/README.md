# media-dl webui

这是 media-dl 的 Vite + React 前端，包含首页、视频下载和音乐搜索下载三个页面。开发环境通过 Vite 将 `/api` 请求代理到本机的 `media-dl server`。

## 开发

先在项目根目录启动 API 服务：

```bash
go run . server
```

再启动前端：

```bash
yarn install
yarn dev
```

开发地址默认为 `http://127.0.0.1:3000`。生产构建使用 `yarn build`，生成的 `dist` 由根目录的 Go HTTP 服务托管。

## 页面

- `/`：项目介绍和视频、音乐两个入口。
- `/video`：调用 HTTP API 解析并下载视频。
- `/music`：按单曲、歌手或专辑搜索，可多选平台，并下载歌曲、歌词、封面或打开播放器。

接口文档见项目根目录的 [HTTP API 文档](../httpserver/README.md)。
