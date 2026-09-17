# media-dl HTTP API

`httpserver` 提供 media-dl 的 HTTP API 和 `webui` 静态文件服务。服务端使用 [olympus/httpserver](https://github.com/ihezebin/olympus) 注册路由、生成 OpenAPI 文档并统一处理请求响应。

## 启动

在项目根目录执行：

```bash
go run . server --port 8080 --web-dir ./webui/dist
```

构建前端后，访问 `http://127.0.0.1:8080/`；OpenAPI 页面为 `http://127.0.0.1:8080/openapi`。

只启动 HTTP API、不启动 WebUI 时，给 `server` 命令增加 `--api-only`：

```bash
go run . server --api-only --port 8080
```

API-only 模式不托管 WebUI 静态文件，并且不注册 WebUI 使用的验证码接口和带验证码接口：`/api/captcha`、`/api/captcha/verify`、`/api/music/search/verified`、`/api/video/info/verified`。普通 HTTP API、在线下载接口和 OpenAPI 页面仍然注册，`--web-dir` 在该模式下不生效。除命令行参数外，也可设置 `MEDIA_DL_API_ONLY=true`。

也可以使用环境变量配置服务：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `MEDIA_DL_PORT` | `8080` | HTTP 端口 |
| `MEDIA_DL_WEB_DIR` | `./webui/dist` | 前端构建目录 |
| `MEDIA_DL_API_ONLY` | `false` | 仅启动 HTTP API，不注册 WebUI 和验证码相关接口 |
| `MEDIA_DL_PROXY` | 空 | HTTP/HTTPS 代理 |
| `MEDIA_DL_COOKIE` | 空 | 直接 Cookie 头 |
| `MEDIA_DL_COOKIES` | 空 | Netscape `cookies.txt` 路径 |

除在线下载接口外，所有接口返回 Olympus 统一结构：

```json
{"code": 0, "message": "OK", "data": {}}
```

失败时 `code` 非 0，具体原因在 `message`。

在线下载接口成功时直接返回二进制附件，并通过 `Content-Disposition` 提供文件名；失败时仍返回上述 JSON 错误结构。服务端只在请求期间使用系统临时文件，响应结束后自动清理，不需要配置下载目录。

## WebUI 验证码

WebUI 点击视频页的“提取视频”或音乐页的“搜索”时，会先调用 `GET /api/captcha` 获取随机的 Slide、Drag-Drop 或 Rotate 挑战，再将操作结果提交到 `POST /api/captcha/verify`。验证成功后服务签发短期 token，WebUI 通过 `X-Captcha-Token` 请求头传给受保护接口。

验证码挑战和凭证仅保存在当前服务进程的内存中，不依赖 Redis 或其他中间件。WebUI 不使用 Cookie 或 localStorage 保存 token，刷新页面后需要重新验证。

WebUI 使用以下受保护接口：

- `POST /api/music/search/verified`
- `POST /api/video/info/verified`

原有的 `POST /api/music/search` 和 `POST /api/video/info` 保留不变，继续作为不带 WebUI 行为验证的 HTTP API 能力提供给调用方。

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
  -d '{"action":"audio","cookie":"KugooID=xxx; ...","song":{"source":"kugou","name":"太阳之子","artist":"歌手","link":"https://www.kugou.com/song/#hash=YOUR_KUGOU_HASH"}}' \
  -o '太阳之子 - 歌手.mp3'
```

解析和下载酷狗歌曲时，Cookie 都放在请求 body 的 `cookie` 字段；搜索接口则按平台放在 `cookies.kugou` 中。Cookie 过期、账号无权限或歌曲属于 VIP/付费资源时，即使请求带 Cookie 也可能无法取得可播放或可下载地址。

该接口成功时直接返回歌曲、封面或歌词文件，不再返回 `data.file_url`。酷狗除 music-lib 原生 hash 链接外，还支持 `m.kugou.com/share/song.html?chain=...` 和两种 `www.kugou.com/mixsong/...html` 链接。

## 视频 API

视频解析器支持以下全部平台。`platform` 可以省略，服务会根据 URL 自动识别。下表使用当前排查过的公开视频或直播间；直播状态、短视频公开状态和平台签名都可能随时间变化。

| 平台 | `platform` | 示例 URL |
| --- | --- | --- |
| 抖音 | `douyin` | `https://v.douyin.com/coDjy36IwNo/` |
| Bilibili | `bilibili` | `https://www.bilibili.com/video/BV1hRNe6wEzV/` |
| 小红书 | `xiaohongshu` | `https://www.xiaohongshu.com/discovery/item/69cf80a20000000022000021?source=webshare&xhsshare=pc_web&xsec_token=ABbwDEgcsNgIRc4tGTKqM9obxwVvdF7AlSh5GHvhaSIhY=&xsec_source=pc_share` |
| 微博 | `weibo` | `https://m.weibo.cn/status/4189191225395228` |
| 优酷 | `youku` | `https://v.youku.com/v_show/id_XNTA2NTA0MjA1Mg==.html` |
| 爱奇艺 | `iqiyi` | `https://www.iqiyi.com/v_19rrny4w8w.html` |
| 西瓜视频 | `xigua` | `https://www.ixigua.com/6996881461559165471` |
| 腾讯视频 | `tencent` | `https://v.qq.com/x/page/q326831cny0.html` |
| YouTube | `youtube` | `https://www.youtube.com/watch?v=dQw4w9WgXcQ` |
| TikTok | `tiktok` | `https://www.tiktok.com/@_halima_07_/video/7492784957073493256` |
| 快手 | `kuaishou` | `https://www.kuaishou.com/short-video/3xegqfwigw73xns?authorId=3xriih3dsywmz6k&streamSource=find&area=homexxbrilliant` |
| 百度视频 | `baidu` | `https://haokan.baidu.com/v?vid=4851961422851197974&pd=&context=` |
| X/Twitter | `twitter` | `https://x.com/SEUNGM1NE/status/2100149744349942038?s=20` |
| 斗鱼 | `douyu` | `https://www.douyu.com/5720533` |
| 虎牙 | `huya` | `https://www.huya.com/lpl` |

### 解析信息

```bash
curl -X POST http://127.0.0.1:8080/api/video/info \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://www.bilibili.com/video/BVxxx"}'
```

也可以将上表中的任意链接放入同一个接口。例如 X/Twitter：

```bash
curl -X POST http://127.0.0.1:8080/api/video/info \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://x.com/SEUNGM1NE/status/2100149744349942038?s=20"}'
```

### 下载视频

```bash
curl -X POST http://127.0.0.1:8080/api/video/download \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://www.bilibili.com/video/BVxxx","format":"mp4"}' \
  -o video.mp4
```

`format` 默认为 `mp4`，`name` 可指定不含扩展名的文件名。在线下载接口只返回视频本体；B 站 DASH、HLS 和多段视频合并需要 `ffmpeg`。

下载接口对上表中的所有平台使用相同格式：将 `url` 替换为对应示例链接即可。例如：

```bash
curl -X POST http://127.0.0.1:8080/api/video/download \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://x.com/SEUNGM1NE/status/2100149744349942038?s=20","format":"mp4"}' \
  -o twitter.mp4
```

视频解析响应中的 `video_url`、`cover_url` 以及格式里的媒体地址始终返回上游原始地址。浏览器预览时由 webui 在请求行为中将地址拼接到同源 `/api/proxy?url=...`，服务端 API 不改写响应数据；服务端执行下载时直接使用原始地址。

## 安全边界

服务端不提供持久下载目录。生产环境建议在反向代理后使用，并自行增加认证和 HTTPS。
