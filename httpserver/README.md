# media-dl HTTP API

`httpserver` 提供 media-dl 的 HTTP API 和 `webui` 静态文件服务。服务端使用 [olympus/httpserver](https://github.com/ihezebin/olympus) 注册路由、生成 OpenAPI 文档并统一处理请求响应。

## 启动

在项目根目录执行：

```bash
go run . server --port 8080 --web-dir ./webui/dist --output ./downloads
```

构建前端后，访问 `http://127.0.0.1:8080/`；OpenAPI 页面为 `http://127.0.0.1:8080/openapi`。

也可以使用环境变量配置服务：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `MEDIA_DL_PORT` | `8080` | HTTP 端口 |
| `MEDIA_DL_WEB_DIR` | `./webui/dist` | 前端构建目录 |
| `MEDIA_DL_OUTPUT_DIR` | `./downloads` | 下载文件目录 |
| `MEDIA_DL_PROXY` | 空 | HTTP/HTTPS 代理 |
| `MEDIA_DL_COOKIE` | 空 | 直接 Cookie 头 |
| `MEDIA_DL_COOKIES` | 空 | Netscape `cookies.txt` 路径 |

所有接口返回 Olympus 统一结构：

```json
{"code": 0, "message": "OK", "data": {}}
```

失败时 `code` 非 0，具体原因在 `message`。

## 音乐 API

音乐平台适配由 [guohuiyuan/music-lib](https://github.com/guohuiyuan/music-lib) 提供。平台值为 `netease`、`qq`、`kugou`、`kuwo`、`migu`、`fivesing`、`qianqian`、`soda`、`jamendo`、`joox`、`bilibili`、`apple`。

### 获取平台

```bash
curl http://127.0.0.1:8080/api/music/platforms
```

### 搜索歌曲

搜索接口使用 POST，所有参数放在 JSON body 中，避免关键词、平台和 Cookie 较长时 URL 超限。`type` 可选 `song`、`artist`、`album`；`platforms` 不传时搜索全部平台；`limit` 是每个平台的结果数。Cookie 按平台放在 `cookies` 对象中。

```bash
curl -X POST http://127.0.0.1:8080/api/music/search \
  -H 'Content-Type: application/json' \
  -d '{"keyword":"晴天","type":"song","platforms":["netease","qq"],"limit":10,"cookies":{"netease":"复制的完整 Cookie"}}'
```

如果 Chrome 已经登录酷狗，按 `F12`（macOS Chrome 为 `⌥⌘I`）打开开发者工具，进入 **Network / 网络**，刷新酷狗页面或播放歌曲，选择一个 `kugou.com` 请求，在 **Headers / 标头 → Request Headers / 请求标头** 中复制完整的 `Cookie` 值。搜索酷狗时将 Cookie 放到 `cookies.kugou`：

```bash
curl -X POST http://127.0.0.1:8080/api/music/search \
  -H 'Content-Type: application/json' \
  -d '{"keyword":"太阳之子","type":"song","platforms":["kugou"],"limit":10,"cookies":{"kugou":"KugooID=xxx; ..."}}'
```

`KugooID=xxx; ...` 只是占位示例，请替换为 Chrome 中复制的完整 Cookie。Cookie 只应在本地命令或受保护的服务端请求中使用，不要提交到 Git。

返回 `data.results` 歌曲数组，每项包含 `id`、`name`、`artist`、`album`、`source`、`link`、`url`、`ext`、`cover` 等字段。部分平台失败时仍会返回可用结果，并在 `data.errors` 中列出错误。

### 解析、歌词和下载

解析搜索结果中的页面链接，适用于搜索结果没有真实音频地址的情况：

```bash
curl -X POST http://127.0.0.1:8080/api/music/resolve \
  -H 'Content-Type: application/json' \
  -d '{"platform":"kugou","url":"https://www.kugou.com/mixsong/bzstdddc.html","cookie":"KugooID=xxx; ..."}'
```

下载接口的 `action` 为 `audio`、`lyrics` 或 `cover`，`song` 使用搜索或解析接口返回的歌曲对象：

```bash
curl -X POST http://127.0.0.1:8080/api/music/download \
  -H 'Content-Type: application/json' \
  -d '{"action":"audio","cookie":"KugooID=xxx; ...","song":{"source":"kugou","name":"太阳之子","artist":"歌手","link":"https://www.kugou.com/song/#hash=YOUR_KUGOU_HASH"}}'
```

解析和下载酷狗歌曲时，Cookie 都放在请求 body 的 `cookie` 字段；搜索接口则按平台放在 `cookies.kugou` 中。Cookie 过期、账号无权限或歌曲属于 VIP/付费资源时，即使请求带 Cookie 也可能无法取得可播放或可下载地址。

响应中的 `data.file_url` 是受服务端输出目录保护的下载地址。酷狗除 music-lib 原生 hash 链接外，还支持 `m.kugou.com/share/song.html?chain=...` 和两种 `www.kugou.com/mixsong/...html` 链接。

## 视频 API

视频解析器支持 Bilibili、抖音、小红书、微博、优酷、爱奇艺、西瓜、腾讯等现有 CLI 平台。`platform` 可以省略，服务会根据 URL 自动识别。

### 解析信息

```bash
curl -X POST http://127.0.0.1:8080/api/video/info \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://www.bilibili.com/video/BVxxx"}'
```

### 下载视频

```bash
curl -X POST http://127.0.0.1:8080/api/video/download \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://www.bilibili.com/video/BVxxx","format":"mp4","cover":true}'
```

`format` 默认为 `mp4`，`name` 可指定不含扩展名的文件名，`cover` 控制是否下载封面。B 站 DASH、HLS 和多段视频合并需要 `ffmpeg`。

视频解析响应中的 `video_url`、`cover_url` 以及格式里的媒体地址始终返回上游原始地址。浏览器预览时由 webui 在请求行为中将地址拼接到同源 `/api/proxy?url=...`，服务端 API 不改写响应数据；服务端执行下载时直接使用原始地址。

## 安全边界

`/api/files/*path` 只允许访问配置的输出目录，服务会拒绝目录穿越路径。生产环境建议在反向代理后使用，并自行增加认证和 HTTPS。
