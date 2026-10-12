import { CheckCircleFilled, CloudDownloadOutlined, ExclamationCircleFilled, FileTextOutlined, KeyOutlined, LinkOutlined, PlayCircleFilled, ThunderboltOutlined } from '@ant-design/icons'
import { Drawer, Input, message, Select, Spin } from 'antd'
import { useEffect, useRef, useState } from 'react'

import { downloadVideo, forgetBehaviorCaptcha, getVideoInfo, isCaptchaRequired, parseCookieFile, proxyURL, triggerFileDownload, type VideoInfo } from '../../api/media'
import { useBehaviorCaptcha } from '../../components/BehaviorCaptcha/useBehaviorCaptcha'

import styles from './index.module.scss'

const platforms = [
  { name: 'YouTube', color: '#ff0033' }, { name: 'TikTok', color: '#111111' },
  { name: 'Bilibili', color: '#00aeec' }, { name: '抖音', color: '#151515' },
  { name: '快手', color: '#ff4906' }, { name: '小红书', color: '#ff2442' },
  { name: '微博', color: '#e6162d' }, { name: '优酷', color: '#24b58a' },
  { name: '爱奇艺', color: '#00be6e' }, { name: '西瓜视频', color: '#ff5d36' },
  { name: '腾讯视频', color: '#18a058' }, { name: '百度视频', color: '#2932e1' },
  { name: 'X / Twitter', color: '#111111' }, { name: '斗鱼', color: '#ff6a00' },
  { name: '虎牙', color: '#ff8a00' },
]

const videoPlatformKeys = ['youtube', 'tiktok', 'bilibili', 'douyin', 'kuaishou', 'xiaohongshu', 'weibo', 'youku', 'iqiyi', 'xigua', 'tencent', 'baidu', 'twitter', 'douyu', 'huya']
const videoPlatformLabels: Record<string, string> = { youtube: 'YouTube', tiktok: 'TikTok', bilibili: 'Bilibili', douyin: '抖音', kuaishou: '快手', xiaohongshu: '小红书', weibo: '微博', youku: '优酷', iqiyi: '爱奇艺', xigua: '西瓜视频', tencent: '腾讯视频', baidu: '百度视频', twitter: 'X / Twitter', douyu: '斗鱼', huya: '虎牙' }
const videoCookieStorageKey = 'media-dl.video-platform-cookies'

function readStoredMap(key: string) {
  if (typeof window === 'undefined') return {}
  try {
    const stored = JSON.parse(window.localStorage.getItem(key) || '{}') as unknown
    if (!stored || typeof stored !== 'object' || Array.isArray(stored)) return {}
    return Object.fromEntries(Object.entries(stored).filter(([, value]) => typeof value === 'string' && value.trim()))
  } catch { return {} }
}

function detectVideoPlatform(raw: string) {
  const value = raw.trim().toLowerCase()
  const rules: Array<[string, string[]]> = [
    ['youtube', ['youtube.com', 'youtu.be']], ['tiktok', ['tiktok.com']], ['bilibili', ['bilibili.com', 'b23.tv']],
    ['douyin', ['douyin.com', 'iesdouyin.com']], ['kuaishou', ['kuaishou.com', 'kuaishouapp.com', 'kwai.com']],
    ['xiaohongshu', ['xiaohongshu.com', 'xhslink.com']], ['weibo', ['weibo.com', 'weibo.cn']], ['youku', ['youku.com']],
    ['iqiyi', ['iqiyi.com', 'iq.com']], ['xigua', ['ixigua.com']], ['tencent', ['v.qq.com', 'qq.com']],
    ['baidu', ['haokan.baidu.com', 'baidu.com']], ['twitter', ['twitter.com', 'x.com']], ['douyu', ['douyu.com']], ['huya', ['huya.com']],
  ]
  return rules.find(([, hosts]) => hosts.some((host) => value.includes(host)))?.[0] || ''
}

function duration(seconds: number) {
  if (!seconds) return '未知时长'
  const minutes = Math.floor(seconds / 60)
  return `${minutes}分${Math.round(seconds % 60)}秒`
}

export default function VideoDownload() {
  const [url, setUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [downloading, setDownloading] = useState(false)
  const [previewError, setPreviewError] = useState(false)
  const [parseError, setParseError] = useState('')
  const [result, setResult] = useState<VideoInfo | null>(null)
  const [videoCookies, setVideoCookies] = useState<Record<string, string>>(() => readStoredMap(videoCookieStorageKey))
  const [videoCookieFiles, setVideoCookieFiles] = useState<Record<string, string>>(() => readStoredMap(`${videoCookieStorageKey}.file-values`))
  const [cookieDraft, setCookieDraft] = useState('')
  const [cookieDrafts, setCookieDrafts] = useState<Record<string, string>>({})
  const [cookieDrawerOpen, setCookieDrawerOpen] = useState(false)
  const [cookieFileNames, setCookieFileNames] = useState<Record<string, string>>(() => readStoredMap(`${videoCookieStorageKey}.files`))
  const [cookiePlatform, setCookiePlatform] = useState('')
  const [cookieDraftFileName, setCookieDraftFileName] = useState('')
  const [cookieDraftFileContent, setCookieDraftFileContent] = useState('')
  const [cookieAddOpen, setCookieAddOpen] = useState(false)
  const audioPreviewRef = useRef<HTMLAudioElement | null>(null)
  const waitingForCaptchaRef = useRef(false)
  const { captcha, captchaLoading, captchaOpen, runWithCaptcha } = useBehaviorCaptcha()

  useEffect(() => {
    if (!waitingForCaptchaRef.current || captchaOpen || captchaLoading) return
    waitingForCaptchaRef.current = false
    setLoading(false)
    setParseError('验证码已取消，未开始解析视频')
  }, [captchaLoading, captchaOpen])

  const parse = async () => {
    if (!url.trim()) {
      waitingForCaptchaRef.current = false
      setLoading(false)
      setResult(null)
      setParseError('请先粘贴视频链接')
      message.warning('请先粘贴视频链接')
      return
    }
    waitingForCaptchaRef.current = false
    setLoading(true)
    try {
      const detectedPlatform = detectVideoPlatform(url)
      const fileValue = detectedPlatform ? videoCookieFiles[detectedPlatform] || '' : ''
      const cookieValue = detectedPlatform && !fileValue ? videoCookies[detectedPlatform] || '' : ''
      const nextResult = await getVideoInfo(url.trim(), detectedPlatform || undefined, cookieValue, fileValue)
      setPreviewError(false)
      setParseError('')
      setResult(nextResult)
      message.success('视频解析完成')
    } catch (error) {
      if (isCaptchaRequired(error)) {
        waitingForCaptchaRef.current = true
        forgetBehaviorCaptcha()
        runWithCaptcha(parse)
        return
      }
      const errorMessage = error instanceof Error ? error.message : '视频解析失败'
      setResult(null)
      setParseError(errorMessage)
      message.error(errorMessage)
    } finally {
      if (!waitingForCaptchaRef.current) setLoading(false)
    }
  }

  const requestParse = () => {
    if (!url.trim()) { message.warning('请先粘贴视频链接'); return }
    setLoading(true)
    setResult(null)
    setParseError('')
    setPreviewError(false)
    waitingForCaptchaRef.current = true
    runWithCaptcha(parse)
  }

  const download = async () => {
    if (!result) return
    setDownloading(true)
    const fileValue = videoCookieFiles[result.platform] || ''
    try { const file = await downloadVideo({ url: result.webpage_url || url.trim(), platform: result.platform, format: 'mp4', cookie: fileValue ? '' : (videoCookies[result.platform] || ''), cookie_file: fileValue }); triggerFileDownload(file); message.success('视频下载已开始') } catch (error) { message.error(error instanceof Error ? error.message : '视频下载失败') } finally { setDownloading(false) }
  }

  const openCookieDrawer = () => { setCookieDrafts({ ...videoCookies }); setCookiePlatform(''); setCookieDraft(''); setCookieDraftFileName(''); setCookieDraftFileContent(''); setCookieAddOpen(false); setCookieDrawerOpen(true) }
  const readCookieFile = async (file: File) => {
    const raw = await file.text()
    const value = parseCookieFile(raw)
    if (!value) { message.warning('未从文件中找到有效的 Netscape Cookie'); return }
    setCookieDraft(value); setCookieDraftFileContent(raw); setCookieDraftFileName(file.name); message.success(`已读取 Cookie 文件：${file.name}`)
  }
  const addCookie = () => {
    if (!cookiePlatform) { message.warning('请先选择视频平台'); return }
    const value = cookieDraft.trim()
    if (!value) { message.warning('请粘贴 Cookie 或选择文件'); return }
    setCookieDrafts((current) => ({ ...current, [cookiePlatform]: value }))
    if (cookieDraftFileContent) setVideoCookieFiles((current) => ({ ...current, [cookiePlatform]: cookieDraftFileContent }))
    else setVideoCookieFiles((current) => { const next = { ...current }; delete next[cookiePlatform]; return next })
    setCookieFileNames((current) => cookieDraftFileName ? ({ ...current, [cookiePlatform]: cookieDraftFileName }) : Object.fromEntries(Object.entries(current).filter(([platform]) => platform !== cookiePlatform)))
    setCookiePlatform(''); setCookieDraft(''); setCookieDraftFileName(''); setCookieDraftFileContent(''); setCookieAddOpen(false)
  }
  const updateCookie = (platform: string, value: string) => setCookieDrafts((current) => ({ ...current, [platform]: value }))
  const removeCookie = (platform: string) => {
    setCookieDrafts((current) => { const next = { ...current }; delete next[platform]; return next })
    setVideoCookieFiles((current) => { const next = { ...current }; delete next[platform]; return next })
    setCookieFileNames((current) => { const next = { ...current }; delete next[platform]; return next })
  }
  const saveCookies = () => {
    const nextCookies = Object.fromEntries(Object.entries(cookieDrafts).map(([platform, value]) => [platform, value.trim()]).filter(([, value]) => value)) as Record<string, string>
    const nextFiles = Object.fromEntries(Object.entries(videoCookieFiles).filter(([platform, value]) => nextCookies[platform] && value.trim())) as Record<string, string>
    const nextNames = Object.fromEntries(Object.entries(cookieFileNames).filter(([platform]) => nextCookies[platform])) as Record<string, string>
    setVideoCookies(nextCookies); setVideoCookieFiles(nextFiles); setCookieFileNames(nextNames)
    try { window.localStorage.setItem(videoCookieStorageKey, JSON.stringify(nextCookies)); window.localStorage.setItem(`${videoCookieStorageKey}.file-values`, JSON.stringify(nextFiles)); window.localStorage.setItem(`${videoCookieStorageKey}.files`, JSON.stringify(nextNames)) } catch { message.warning('Cookie 已应用，但写入 localStorage 失败') }
    setCookieDrawerOpen(false); message.success(`已保存 ${Object.keys(nextCookies).length} 个平台 Cookie`)
  }

  const previewFormat = result?.formats.find((format) => format.url === result.video_url)
  const previewAudioURL = previewFormat?.audio_url || ''
  const syncPreviewAudio = (action: 'play' | 'pause' | 'seek' | 'ended', video?: HTMLVideoElement) => {
    const audio = audioPreviewRef.current
    if (!audio || !video) return
    if (action === 'play') { audio.currentTime = video.currentTime; void audio.play().catch(() => undefined) }
    if (action === 'pause') audio.pause()
    if (action === 'seek') audio.currentTime = video.currentTime
    if (action === 'ended') { audio.pause(); audio.currentTime = 0 }
  }

  return <div className={styles.page}>
    <div className={styles.pageIntro}><div className={styles.overline}>VIDEO / 01</div><h1>视频下载</h1><p>复制分享链接，提取你想要的清晰度。</p></div>
    <section className={styles.workspace}>
      <div className={styles.panelHead}><div><span className={styles.panelEyebrow}>QUICK EXTRACT</span><h2>粘贴链接开始</h2></div><div className={styles.panelTools}><button type="button" className={styles.cookieButton} onClick={openCookieDrawer}><KeyOutlined />视频 Cookie</button><span className={styles.step}>STEP 01 <b>→</b> 02</span></div></div>
      <div className={styles.inputRow}><div className={styles.inputWrap}><LinkOutlined /><input value={url} onChange={(event) => setUrl(event.target.value)} onKeyDown={(event) => event.key === 'Enter' && requestParse()} placeholder="粘贴 YouTube、TikTok、Bilibili 等视频链接" aria-label="视频链接" /><button type="button" aria-label="清空链接" onClick={() => { setUrl(''); setResult(null); setParseError(''); setPreviewError(false) }}>×</button></div><button type="button" className={styles.parseButton} onClick={requestParse} disabled={loading || captchaLoading}>{captchaLoading ? <><Spin size="small" />验证中...</> : loading ? <><Spin size="small" />解析中...</> : <><ThunderboltOutlined />提取视频</>}</button></div>
      <div className={styles.hint}><CheckCircleFilled />支持分享口令自动识别 · 解析结果仅在本地展示</div>
    </section>
    {(loading || parseError || result) && <section className={styles.previewSection} aria-label="视频解析状态">
      {result && <div className={styles.previewHeader}>
        <div className={styles.previewInfo}>
          <span className={styles.resultTag}>解析成功 · {result.platform}</span>
          <h2 id="video-preview-title">{result.title || '已识别的视频资源'}</h2>
          <p className={styles.description}>{result.description || '暂无描述'}</p>
          <div className={styles.metadata}>
            <span>{result.author || '未知作者'}</span>
            <span>{duration(result.duration)}</span>
            <span>{result.formats.length ? `${result.formats.length} 种格式` : '默认格式'}</span>
            {result.id && <span>ID：{result.id}</span>}
          </div>
          {(result.webpage_url || url) && <a className={styles.sourceLink} href={result.webpage_url || url} target="_blank" rel="noreferrer"><LinkOutlined />查看原页面</a>}
        </div>
        <button type="button" className={styles.downloadButton} onClick={download} disabled={downloading}>{downloading ? <Spin size="small" /> : <CloudDownloadOutlined />}{downloading ? '下载中' : '下载视频'}</button>
      </div>}
      <div className={styles.videoStage}>
        {loading ? <div className={styles.videoEmpty} aria-live="polite"><Spin size="large" /><strong>正在解析视频…</strong><span>请稍候，视频区域将在解析完成后更新。</span></div> : parseError ? <div className={`${styles.videoEmpty} ${styles.videoError}`} role="alert"><ExclamationCircleFilled /><strong>视频解析失败</strong><span>{parseError}</span></div> : result && result.platform === 'youtube' && result.id ? <iframe title={result.title || 'YouTube 视频预览'} src={`https://www.youtube.com/embed/${encodeURIComponent(result.id)}`} allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowFullScreen /> : result && result.platform === 'tiktok' && result.id ? <iframe title={result.title || 'TikTok 视频预览'} src={`https://www.tiktok.com/player/v1/${encodeURIComponent(result.id)}?lang=zh-CN`} allow="autoplay; fullscreen" allowFullScreen /> : result && (result.video_url && !previewError ? <><video controls playsInline preload="metadata" poster={proxyURL(result.cover_url)} src={proxyURL(result.video_url)} onPlay={(event) => syncPreviewAudio('play', event.currentTarget)} onPause={(event) => syncPreviewAudio('pause', event.currentTarget)} onSeeking={(event) => syncPreviewAudio('seek', event.currentTarget)} onEnded={(event) => syncPreviewAudio('ended', event.currentTarget)} onError={() => setPreviewError(true)}>您的浏览器不支持视频播放。</video>{previewAudioURL && <audio ref={audioPreviewRef} preload="auto" src={proxyURL(previewAudioURL)} onError={() => setPreviewError(true)} />}</> : <div className={styles.videoEmpty}><PlayCircleFilled /><strong>{result.video_url ? '当前视频地址暂不支持直接预览' : '当前结果没有可预览的视频地址'}</strong><span>可以尝试点击右上角下载视频。</span></div>)}
      </div>
    </section>}
    <section className={styles.platformSection}><div className={styles.sectionTitle}><span>SUPPORTED SOURCES</span><h2>支持的平台</h2></div><div className={styles.platformGrid}>{platforms.map((platform) => <div className={styles.platform} key={platform.name}><span className={styles.platformDot} style={{ background: platform.color }} />{platform.name}<span className={styles.platformArrow}>↗</span></div>)}</div></section>
    <aside className={styles.tip}><span>TIP</span><div><strong>链接解析</strong><p>支持完整网页链接和大部分平台的分享文案。下载前会保留原始画质选项。</p></div></aside>
    <Drawer title="视频 Cookie" placement="right" width={420} open={cookieDrawerOpen} onClose={() => setCookieDrawerOpen(false)}>
      <div className={styles.cookieDrawer}>
        <div className={styles.cookieNotice}><KeyOutlined /><span>Cookie 按视频平台分别保存。解析链接时会自动带上对应平台的 Cookie，并通过请求 body 发送。</span></div>
        <div className={styles.cookieSectionHead}><strong>已添加的平台</strong><span>{Object.keys(cookieDrafts).length} 个</span></div>
        {Object.keys(cookieDrafts).length ? <div className={styles.cookieEntries}>{Object.entries(cookieDrafts).map(([platform, value]) => <div className={styles.cookieEntry} key={platform}>
          <div className={styles.cookieEntryHead}><div className={styles.cookiePlatform}><div><strong>{videoPlatformLabels[platform] || platform}</strong><small>{platform}</small></div></div><button type="button" className={styles.cookieRemoveButton} onClick={() => removeCookie(platform)} aria-label={`移除${videoPlatformLabels[platform] || platform} Cookie`}>×</button></div>
          {cookieFileNames[platform] || videoCookieFiles[platform] ? <><div className={styles.cookieFileSelected}><FileTextOutlined />{cookieFileNames[platform] || 'Cookie 文件'}<span>已选文件优先</span></div><pre style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', maxHeight: 160, overflow: 'auto', margin: 0 }} aria-label="Cookie 文件内容">{videoCookieFiles[platform] || value}</pre></> : <Input.TextArea value={value} onChange={(event) => updateCookie(platform, event.target.value)} rows={4} placeholder="粘贴浏览器请求头中的完整 Cookie 字符串" spellCheck={false} />}
        </div>)}</div> : <div className={styles.cookieEmpty}>还没有添加平台 Cookie</div>}
        {videoPlatformKeys.some((platform) => !cookieDrafts[platform]?.trim()) ? <div className={styles.cookieAddArea}>
          <button type="button" className={styles.cookieAddButton} onClick={() => setCookieAddOpen((open) => !open)}>＋添加平台 Cookie</button>
          {cookieAddOpen && <div className={styles.cookieAddForm}>
            <label className={styles.cookieLabel} htmlFor="video-cookie-platform">选择未添加的平台</label>
            <Select id="video-cookie-platform" value={cookiePlatform || undefined} placeholder="选择平台" onChange={(platform) => { setCookiePlatform(platform); setCookieDraft(''); setCookieDraftFileName(''); setCookieDraftFileContent('') }} options={videoPlatformKeys.filter((platform) => !cookieDrafts[platform]?.trim()).map((platform) => ({ value: platform, label: videoPlatformLabels[platform] }))} />
            {cookiePlatform && <><label className={styles.cookieFileButton} htmlFor="video-cookie-file"><FileTextOutlined />选择 Cookie 文件</label><input id="video-cookie-file" type="file" accept=".txt,text/plain" hidden onChange={(event) => { const file = event.target.files?.[0]; if (file) void readCookieFile(file); event.currentTarget.value = '' }} />{cookieDraftFileName ? <><div className={styles.cookieFileSelected}><FileTextOutlined />{cookieDraftFileName}<span>已选文件优先</span></div><pre style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', maxHeight: 160, overflow: 'auto', margin: 0 }} aria-label="待保存 Cookie 文件内容">{cookieDraftFileContent}</pre></> : <><label className={styles.cookieLabel} htmlFor="video-cookie-value">粘贴 Cookie</label><Input.TextArea id="video-cookie-value" value={cookieDraft} onChange={(event) => setCookieDraft(event.target.value)} rows={6} placeholder="粘贴浏览器请求头中的完整 Cookie 字符串" spellCheck={false} /></>}<button type="button" className={styles.cookieAddConfirm} onClick={addCookie}>＋添加到待保存列表</button></>}
          </div>}
        </div> : <div className={styles.cookieAllAdded}>已添加全部视频平台</div>}
        <button type="button" className={styles.cookieSaveButton} onClick={saveCookies}><KeyOutlined />保存全部 Cookie</button>
        <div className={styles.cookieGuide}><h3>如何获取</h3><ol><li>打开目标视频平台并确保页面可以正常访问。</li><li>在开发者工具 Network 面板刷新页面。</li><li>复制请求头中的完整 Cookie，不要只复制一个字段。</li></ol><p>抖音遇到 403 时，Cookie 通常需要包含 UIFID/UIFID_TEMP；Cookie 过期后重新获取即可。</p></div>
      </div>
    </Drawer>
    {captcha}
  </div>
}
