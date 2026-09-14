# media-dl

用 Go 实现的轻量 CLI：解析多平台分享链接并下载视频，也支持聚合音乐搜索与下载。视频协议参考 [yt-dlp](https://github.com/yt-dlp/yt-dlp)、[you-get](https://github.com/soimort/you-get) 与 [dy-cli](https://github.com/Youhai020616/douyin)，按接口重写，而非翻译 Python；音乐能力基于 [guohuiyuan/music-lib](https://github.com/guohuiyuan/music-lib)。

## 构建

本机（当前系统 / 架构）：

```bash
go build -o media-dl .
```

安装到本机全局（写入 `$GOPATH/bin`，需确保该目录已在 `PATH` 中；二进制名取自模块路径最后一段 `media-dl`）：

```bash
go install .
```

之后可在任意目录直接使用 `media-dl`；代码更新后重新执行一次即可。

交叉编译 Linux（纯 Go，无需本机交叉工具链；`CGO_ENABLED=0` 保证静态链接）：

```bash
# amd64（常见 x86_64 服务器 / 云主机）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./dist/media-dl-linux-amd64 .

# arm64（ARM 服务器、树莓派 64 位、Apple Silicon 上的 Linux 等）
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ./dist/media-dl-linux-arm64 .
```

依赖：Go 1.25.10+。命令行由 [Cobra](https://github.com/spf13/cobra) 提供；音乐平台适配依赖开源项目 [guohuiyuan/music-lib](https://github.com/guohuiyuan/music-lib)。B 站 DASH 分离流、优酷/爱奇艺/腾讯的 HLS 或多段视频，合并时需要本机已安装 `ffmpeg`。

## 总览

视频命令：

```text
media-dl video <command> <platform> <url> [flags]
```

音乐命令：

```text
media-dl music search [keyword] [flags]
media-dl music download <platform> <url> [flags]
```

### 代码结构

```text
internal/
  music/                  音乐搜索、平台适配封装和歌曲下载
  video/
    extractor/            视频平台解析器
    model/                视频统一信息模型
    downloader/           视频文件下载和 ffmpeg 合并
  httpx/                  视频和音乐共用的 HTTP、Cookie、代理能力
  util/                   视频和音乐共用的文件名等工具
```

## 公共参数（video / music）

以下参数是根命令的持久参数，`video info`、`video download`、`music search`、`music download` 均可使用。它们的参数名相同，但在两个业务域中注入请求的方式略有不同。

| 参数 | 默认值 | video 行为 | music 行为 |
| --- | --- | --- | --- |
| `--proxy` | 空 | 通过视频 HTTP 客户端代理解析、下载、封面和 DASH/HLS 请求。 | 配置 `music-lib` 和音乐下载使用的 HTTP transport，影响搜索、歌曲解析、下载、封面和歌词请求。 |
| `--cookies` | 空 | 读取 Netscape 格式 `cookies.txt`，按 Cookie 的域名和路径规则注入视频请求，用于登录态、VIP、412 和风控场景。 | 读取同一文件，将 Cookie 传入音乐库；音乐页面、音频和封面请求也使用对应 Cookie。 |
| `--cookie` | 空 | 将直接传入的 `Cookie` 头复制到支持的视频平台域名，用于临时登录态、VIP 或风控调试。 | 将原始 `Cookie` 头传入音乐库和音乐 HTTP 请求；Apple Music 可传 `media-user-token`，也可传音乐库支持的 `token`。 |
| `-h, --help` | — | 显示当前 video 命令或子命令帮助。 | 显示当前 music 命令或子命令帮助。 |

`--cookie` 和 `--cookies` 可以同时传入；实现会合并两者。Cookie 通常具有时效性，使用浏览器导出的登录态时不要提交到 Git。

错误信息：video `info` 将失败信息写到 stdout JSON 的 `err_msg`（`success: false`），进程非 0 退出；video/music 下载过程信息写到 **stderr**；music `search` 输出 JSON，部分平台失败时会保留在 `errors` 字段。

## 音乐功能

音乐命令统一以 `media-dl music` 开头，由开源项目 `github.com/guohuiyuan/music-lib` 提供平台适配。本项目负责命令参数、跨平台聚合、Cookie/代理传递、文件保存和酷狗扩展链接转换。

| 平台值 | 平台名称 | 常用别名 |
| --- | --- | --- |
| `netease` | 网易云音乐 | `163`、`网易云音乐` |
| `qq` | QQ音乐 | `qqmusic`、`QQ音乐` |
| `kugou` | 酷狗音乐 | `酷狗音乐` |
| `kuwo` | 酷我音乐 | `酷我音乐` |
| `migu` | 咪咕音乐 | `咪咕音乐` |
| `fivesing` | 5sing | `5sing` |
| `qianqian` | 千千音乐 | `千千音乐` |
| `soda` | 汽水音乐 | `汽水音乐` |
| `bilibili` | Bilibili | `bili` |
| `apple` | Apple Music | `applemusic` |

### `music search` — 搜索歌曲

不传 `--platform` 时，会并发搜索上表中的全部平台；传入后只搜索指定平台。歌手和歌名可以分别通过参数传入，也可以直接使用一个关键词。

```bash
# 搜索所有平台
./media-dl music search "周杰伦 晴天"

# 按歌手和歌名搜索所有平台
./media-dl music search --artist "周杰伦" --title "晴天"

# 只搜索网易云和 QQ 音乐
./media-dl music search --platform netease,qq "周杰伦 晴天"

# 平台也支持中文名，限制每个平台返回数量
./media-dl music search -p 网易云音乐 -l 5 --singer "周杰伦" -t "晴天"
```

参数：

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `[keyword]` | 空 | 通用搜索关键词，可与 `--artist` / `--title` 组合 |
| `-p, --platform` | 全部平台 | 平台名，可重复传入或使用逗号分隔 |
| `-a, --artist` / `--singer` | 空 | 歌手名 |
| `-t, --title` | 空 | 歌曲名 |
| `-l, --limit` | `10` | 每个平台最多返回的结果数 |

输出为 JSON。`results` 中的每项包含 `id`、`name`、`artist`、`album`、`source`、`link`、`url`、`ext`、`cover` 等字段；部分平台失败时，错误会保留在 `errors` 中，只要仍有搜索结果就不会导致命令失败。

### `music download` / `music dl` — 下载歌曲

命令会先调用对应平台的 `Parse` 解析单曲链接，再调用 `GetDownloadURL` 获取音频地址并保存到本地。支持同时下载封面和歌词：

```bash
./media-dl music download netease "https://music.163.com/#/song?id=123456" -o ./downloads
./media-dl music dl qq "https://y.qq.com/n/ryqq/songDetail/xxx" --cover --lyrics
./media-dl music download kuwo "https://www.kuwo.cn/play_detail/123456" -o ./downloads -n "我的歌曲"
```

参数：

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-o, --output` | `./downloads` | 保存目录，不存在时自动创建 |
| `-n, --name` | 自动生成 | 输出文件名，不含扩展名；默认使用“歌名 - 歌手” |
| `--cover` | `false` | 同时下载封面为 `.jpg` |
| `--lyrics` | `false` | 同时获取并保存歌词为 `.lrc` |

#### 酷狗 URL 解析

酷狗除 `music-lib` 原生支持的 hash 链接外，还会先请求页面并从 `dataFromSmarty[].hash` 提取 hash，支持以下三类链接：

```bash
./media-dl music download kugou "https://m.kugou.com/share/song.html?chain=2wKoD7fG5V2"
./media-dl music download kugou "https://www.kugou.com/mixsong/bzstdddc.html"
./media-dl music download kugou "https://www.kugou.com/mixsong/7283tjfe.html?fromsearch=%E5%85%B3%E4%BA%8E%E4%BD%A0"
```

解析后会转换为 `https://www.kugou.com/song/#hash=<32位hash>`，再交给 `music-lib/kugou` 继续获取歌曲信息和下载地址。原生 hash 链接仍可直接使用。

#### Apple Music 限制

Apple Music 的搜索和单曲解析可用；当前上游 `music-lib` 的 `GetDownloadURL` 返回 Apple Music 预览地址。完整歌曲涉及 DRM，不能由本命令直接解密下载；需要使用具备相应授权与解密流程的专用工具（例如 `gamdl`）。

## 视频功能

视频命令统一以 `media-dl video` 开头。视频平台解析器、统一信息模型和下载器均收拢在 `internal/video` 下；`internal/httpx` 与 `internal/util` 仅存放视频和音乐共同使用的基础能力。

### 支持平台

| 平台值           | 别名              | 典型链接                                                                 |
| ------------- | --------------- | -------------------------------------------------------------------- |
| `douyin`      | `dy`            | `v.douyin.com`、`www.douyin.com/video/...`                            |
| `bilibili`    | `bili`、`b23`    | `www.bilibili.com/video/BV...`、`b23.tv/...`                          |
| `xiaohongshu` | `xhs`           | `www.xiaohongshu.com/explore/...`、`discovery/item/...`、`xhslink.com` |
| `weibo`       | `wb`、`微博`       | `weibo.com/{uid}/{id}`、`m.weibo.cn/status/...`、`weibo.com/tv/show/`  |
| `youku`       | `yk`、`优酷`       | `v.youku.com/v_show/id_...`、`play.tudou.com/v_show/id_...`           |
| `iqiyi`       | `iq`、`爱奇艺`      | `www.iqiyi.com/v_....html`                                           |
| `xigua`       | `ixigua`、`西瓜视频` | `www.ixigua.com/{id}`、`v.ixigua.com/...`                             |
| `tencent`     | `qq`、`腾讯视频`     | `v.qq.com/x/page/...`、`v.qq.com/x/cover/.../...`                     |



### `video info` — 只获取信息

解析链接并打印统一 JSON 到 **stdout**，**不下载文件**。

### 用法

```bash
./media-dl video info <platform> <url> [--proxy ...] [--cookies ...] [--cookie ...]
```



### 示例

```bash
./media-dl video info douyin "https://v.douyin.com/coDjy36IwNo/"
./media-dl video info bilibili "https://www.bilibili.com/video/BV1hRNe6wEzV/?share_source=copy_web&vd_source=db31d99c9cc84c67d33c33e7f08c6620"
./media-dl video info xiaohongshu "https://www.xiaohongshu.com/discovery/item/69cf80a20000000022000021?source=webshare&xhsshare=pc_web&xsec_token=ABbwDEgcsNgIRc4tGTKqM9obxwVvdF7AlSh5GHvhaSIhY=&xsec_source=pc_share"
./media-dl video info weibo "https://weibo.com/tv/show/1034:4797699866951785"
./media-dl video info youku "https://v.youku.com/v_show/id_XNTA2NTA0MjA1Mg==.html"
./media-dl video info iqiyi "https://www.iqiyi.com/v_19rrny4w8w.html"
./media-dl video info xigua "https://www.ixigua.com/6996881461559165471"
./media-dl video info tencent "https://v.qq.com/x/page/q326831cny0.html"
```



### 输出

- **格式**：缩进 JSON。除 `err_msg` 外字段集合固定（缺省值用空字符串 / `0` / `[]` / `false`，不会省略键）。`err_msg` **仅失败时出现**。
- **出口**：stdout；可用管道：`./media-dl video info dy "URL" | jq .video_url`
- **失败**：仍输出 JSON（`success: false` + `err_msg`），退出码非 0。



#### 顶层字段


| 字段            | 类型     | 含义                                                                                                  |
| ------------- | ------ | --------------------------------------------------------------------------------------------------- |
| `success`     | bool   | 是否解析成功                                                                                              |
| `err_msg`     | string | **仅失败时存在**：具体失败原因（如下线、VIP、风控、链接无法识别）                                                              |
| `platform`    | string | 平台标识：`douyin` / `bilibili` / `xiaohongshu` / `weibo` / `youku` / `iqiyi` / `xigua` / `tencent`      |
| `id`          | string | 平台侧内容 ID。抖音为 `aweme_id`；B 站为 BV 号（多分 P 时可能带 `_pN`）；小红书为笔记 ID                                        |
| `title`       | string | 标题；抖音常与文案相同，小红书无标题时可能回退到描述或 ID                                                                      |
| `description` | string | 描述 / 文案；没有则为 `""`                                                                                   |
| `author`      | string | 作者昵称                                                                                                |
| `author_id`   | string | 作者平台 ID（抖音 uid、B 站 mid、小红书 userId）；没有则为 `""`                                                        |
| `duration`    | number | 时长，单位**秒**；解析不到时为 `0`                                                                               |
| `cover_url`   | string | 封面图 URL；没有则为 `""`                                                                                   |
| `webpage_url` | string | 规范化后的页面链接（便于打开或二次请求）                                                                                |
| `video_url`   | string | **优选**下载地址，等于对 `formats` 按质量/是否一体流评分后的最佳项的 `url`。一般可直接拿去下载；B 站若该项是 DASH 纯视频，还需配合对应项的 `audio_url` 合并 |
| `formats`     | array  | 全部可用流列表，元素结构见下表；无流时为 `[]`                                                                           |




#### `formats[]` 字段


| 字段                  | 类型     | 含义                                                                             |
| ------------------- | ------ | ------------------------------------------------------------------------------ |
| `format_id`         | string | 流标识。例如抖音 `no_watermark`、B 站 `durl_16` / `dash_64`、小红书 `HD` / `origin`          |
| `url`               | string | 该流的媒体地址（可能带签名，有时效）                                                             |
| `ext`               | string | 建议扩展名，如 `mp4`、`jpg`                                                            |
| `quality`           | string | 清晰度标签（如 `720p`、`HD`）；未知为 `""`                                                  |
| `width` / `height`  | number | 分辨率；未知为 `0`                                                                    |
| `filesize`          | number | 字节大小；未知为 `0`                                                                   |
| `vcodec` / `acodec` | string | 视/音频编码名；未知或无对应轨时可能为 `""` / `none`                                              |
| `has_video`         | bool   | 是否含视频轨                                                                         |
| `has_audio`         | bool   | 是否含音频（一体流，或可通过 `audio_url` 配齐）                                                 |
| `audio_url`         | string | DASH **分离音轨**地址。非空表示 `url` 多为纯视频，下载时需与本字段合并（`download` 会自动用 ffmpeg）；一体流则为 `""` |




#### 输出示例（结构示意）

```json
{
  "success": true,
  "platform": "douyin",
  "id": "7623740581354087625",
  "title": "...",
  "description": "...",
  "author": "...",
  "author_id": "...",
  "duration": 270.631,
  "cover_url": "https://...",
  "webpage_url": "https://www.douyin.com/video/...",
  "video_url": "https://...",
  "formats": [
    {
      "format_id": "no_watermark",
      "url": "https://...",
      "ext": "mp4",
      "quality": "",
      "width": 0,
      "height": 0,
      "filesize": 0,
      "vcodec": "",
      "acodec": "",
      "has_audio": true,
      "has_video": true,
      "audio_url": ""
    }
  ]
}
```

失败示例：

```json
{
  "success": false,
  "err_msg": "[iqiyi] 视频已下线: 美国德州空中惊现奇异云团 酷似UFO",
  "platform": "iqiyi",
  "id": "",
  "title": "",
  "description": "",
  "author": "",
  "author_id": "",
  "duration": 0,
  "cover_url": "",
  "webpage_url": "https://www.iqiyi.com/v_19rrojlavg.html",
  "video_url": "",
  "formats": []
}
```

---



### `video download` / `video dl` — 下载

先解析（逻辑与 `info` 相同），再按优选格式下载到本地。

### 用法

```bash
./media-dl video download <platform> <url> [flags]
./media-dl video dl <platform> <url> [flags]
```



### 参数


| 参数                                   | 默认    | 说明                          | 对应输出                               |
| ------------------------------------ | ----- | --------------------------- | ---------------------------------- |
| `-o, --output`                       | `.`   | 保存目录（不存在会创建）                | 文件写到该目录                            |
| `-f, --format`                       | `mp4` | 输出容器（`mp4` / `mkv` 等）       | 主视频扩展名；DASH 合并 / remux 时传给 ffmpeg  |
| `-n, --name`                         | 空     | 文件名（**不含**扩展名）；空则用标题消毒后的文件名 | `{name}.{format}`，封面为 `{name}.jpg` |
| `--cover`                            | false | 同时下载封面                      | 额外写出封面文件                           |




### 示例

```bash
./media-dl video download douyin "https://v.douyin.com/coDjy36IwNo/" -o ./downloads -f mp4
./media-dl video download bilibili "https://www.bilibili.com/video/BV1hRNe6wEzV/?share_source=copy_web&vd_source=db31d99c9cc84c67d33c33e7f08c6620" -o ./downloads -f mp4
./media-dl video download xiaohongshu "https://www.xiaohongshu.com/discovery/item/69cf80a20000000022000021?source=webshare&xhsshare=pc_web&xsec_token=ABbwDEgcsNgIRc4tGTKqM9obxwVvdF7AlSh5GHvhaSIhY=&xsec_source=pc_share" -o ./downloads -f mp4

./media-dl video dl dy "https://v.douyin.com/coDjy36IwNo/" --cover
./media-dl video dl bili "https://www.bilibili.com/video/BV1hRNe6wEzV/?share_source=copy_web&vd_source=db31d99c9cc84c67d33c33e7f08c6620" --cookies cookies.txt
./media-dl video dl weibo "https://m.weibo.cn/status/4189191225395228" -o ./downloads
./media-dl video dl tencent "https://v.qq.com/x/page/q326831cny0.html" -o ./downloads
```



### 输出


| 通道         | 内容                                                                        |
| ---------- | ------------------------------------------------------------------------- |
| **stderr** | 进度：`解析中...`、平台/ID/标题摘要、下载百分比；结束时 `完成: <视频路径>`，若 `--cover` 成功还有 `封面: <路径>` |
| **文件系统**   | 主文件：`<output>/<name>.<format>`；`--cover` 时另有 `<output>/<name>.jpg`        |


下载选用规则与 `info` 的 `video_url` 一致：优先音视频一体流，否则 DASH 视频 + `audio_url` 经 ffmpeg 合并。

### 视频平台说明



#### 抖音

- 短链 `v.douyin.com` 跟随重定向取 `aweme_id`
- 优先尝试 `iesdouyin.com/share/video` SSR（`_ROUTER_DATA`）；若平台已去掉内嵌视频数据，则回退到 Web `aweme/detail` + `a_bogus`（自动注册 `ttwid`）
- 播放地址 `playwm` → `play` 尽量无水印
- 可提取封面 `origin_cover` / `cover`
- 若 detail 仍为空，可用浏览器导出 Cookie：`--cookies cookies.txt`



#### 哔哩哔哩

参考 yt-dlp 的处理：

- WBI 签名（`wts` / `w_rid`）
- 请求带 `Referer` + `Origin: https://www.bilibili.com`（缓解 [412](https://github.com/yt-dlp/yt-dlp/issues/14830)）
- `playurl` 附带 `dm_img_*` 指纹参数
- 自动注入 `buvid3`
- 优先一体流 `durl`，否则 DASH + ffmpeg 合并
- 仍 412 时用浏览器导出 Cookie：`--cookies cookies.txt`



#### 小红书

- 支持 `xhslink.com` 短链
- 解析页面 `window.__INITIAL_STATE__`
- 优先 `originVideoKey` 原片，否则取 `masterUrl` / `backupUrls`
- 完整分享链接建议保留 `xsec_token`；打不开时加 `--cookies`



#### 微博

参考 yt-dlp：访客 Cookie + `ajax/statuses/show`；微博 TV / H5 另参考 lux。

- 支持 `weibo.com/{uid}/{mblogid}`、`m.weibo.cn/status|detail/{id}`、`weibo.com/tv/show/{oid}`、`video.weibo.com/show?fid=`
- 未登录时自动走 `passport.weibo.com/visitor/genvisitor` 拿访客 Cookie
- 播放地址来自 `playback_list` / TV `urls`
- 403 时加 `--cookies`，下载时带 `Referer: https://weibo.com/`



#### 优酷

参考 yt-dlp / you-get：`ups.youku.com/ups/get.json`。

- 支持 `v.youku.com/v_show/id_...`、土豆 `play.tudou.com`
- 自动取 `cna`（`log.mmstat.com/eg.js`）并尝试 `ccode` 0564 / 0502
- 优先分段 `cdn_url`，否则 HLS `m3u8_url`（需 ffmpeg）
- 版权/地区限制或加密视频可能需要国内网络与 `--cookies`



#### 爱奇艺

参考 lux 的 VPS 签名，以及 yt-dlp / you-get 的移动端 `tmts`。

- SPA 页不再内嵌 `data-player-tvid`：用 `accelerator.js`（Referer=播放页）取元数据，或从 `v_/w_/p_` slug 还原 tvid
- 再用 `baseinfo` 补 `vid`；优先 `cache.video.iqiyi.com/vps` 分段 MP4，失败回退 `tmts` m3u8
- 已下线内容会直接报「视频已下线」；VIP 正片通常只能下试看段，可加 `--cookies`



#### 西瓜视频

PC 页有 antibot，主路径走头条移动详情 `m.toutiao.com/i{id}/info/`，再用 `play_auth_token_v2` 调火山引擎 `GetPlayInfo`。

- 支持 `www.ixigua.com/{id}`、`m.ixigua.com/video/{id}`、`v.ixigua.com` 短链、部分头条链接
- 自动注册 `ttwid`；有浏览器 Cookie 时仍可回退页面 `SSR_HYDRATED_DATA`
- 播放地址带 `Referer: https://www.ixigua.com/`



#### 腾讯视频

综合 [you-get](https://github.com/soimort/you-get) 的 `getinfo` + `getkey` 分段 MP4，以及 yt-dlp 的 `cKey` + HLS `getvinfo`。

- 支持 `v.qq.com/x/page/{vid}.html`、`v.qq.com/x/cover/{cid}/{vid}.html`
- 先走 `vv.video.qq.com/getinfo`（platform 11 / 4100201），再按清晰度 `getkey` 拼 `vkey`
- 失败则用 AES-CBC 生成 `cKey` 请求 `h5vv6.video.qq.com/getvinfo`（HLS，需 ffmpeg）
- 剧集封面页（仅 cover、无 vid）不支持整季下载；VIP 内容需登录 Cookie



## Cookie 文件

B 站出现 **412**、小红书笔记页打不开、西瓜/微博解析失败、腾讯/爱奇艺 VIP 内容无法解析，或音乐平台需要登录态时，把浏览器里已能正常访问对应站点的 Cookie 导出给 CLI 使用。

1. 用 Chrome / Edge / Firefox 打开对应站点并确认能正常访问内容（**不必登录账号**，有时仅打开过首页产生的 Cookie 也够；登录通常更稳）。
2. 安装 Cookie 导出扩展，例如：
  - Chrome：[Get cookies.txt LOCALLY](https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc)
  - 或 [Cookie-Editor](https://chromewebstore.google.com/detail/cookie-editor/hlkenndednhfkekhgcdicdfddnkalmdm)（导出为 Netscape 格式）
3. 在目标站点标签页点击扩展 → **Export / 导出** → 选择 **Netscape** 格式 → 保存为项目目录下的 `cookies.txt`。
4. 调用时带上文件：

```bash
./media-dl video info bilibili "https://www.bilibili.com/video/BV1hRNe6wEzV/?share_source=copy_web&vd_source=db31d99c9cc84c67d33c33e7f08c6620" --cookies cookies.txt
./media-dl video dl xhs "https://www.xiaohongshu.com/discovery/item/69cf80a20000000022000021?source=webshare&xhsshare=pc_web&xsec_token=ABbwDEgcsNgIRc4tGTKqM9obxwVvdF7AlSh5GHvhaSIhY=&xsec_source=pc_share" --cookies cookies.txt
./media-dl music search -p qq --artist "周杰伦" --title "晴天" --cookies cookies.txt
```

也可用浏览器开发者工具复制整段 Cookie 头：

```bash
./media-dl video info bili "URL" --cookie "SESSDATA=xxx; bili_jct=xxx; buvid3=xxx"
./media-dl music search -p apple "Love Story" --cookie "media-user-token=xxx"
```

`cookies.txt` 含登录态，**不要提交到 Git**（已在 `.gitignore` 中忽略）。

## License

MIT
