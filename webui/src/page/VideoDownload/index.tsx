import { CheckCircleFilled, CloudDownloadOutlined, ExclamationCircleFilled, KeyOutlined, LinkOutlined, PlayCircleFilled, ThunderboltOutlined } from '@ant-design/icons'
import { Drawer, Input, message, Spin } from 'antd'
import { useEffect, useRef, useState } from 'react'

import { downloadVideo, forgetBehaviorCaptcha, getVideoInfo, isCaptchaRequired, proxyURL, triggerFileDownload, type VideoInfo } from '../../api/media'
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

const videoCookieStorageKey = 'media-dl.video-cookie'

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
  const [videoCookie, setVideoCookie] = useState('')
  const [cookieDraft, setCookieDraft] = useState('')
  const [cookieDrawerOpen, setCookieDrawerOpen] = useState(false)
  const waitingForCaptchaRef = useRef(false)
  const { captcha, captchaLoading, captchaOpen, runWithCaptcha } = useBehaviorCaptcha()

  useEffect(() => {
    try {
      const stored = window.localStorage.getItem(videoCookieStorageKey) || ''
      setVideoCookie(stored)
      setCookieDraft(stored)
    } catch { /* localStorage unavailable */ }
  }, [])

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
      const nextResult = await getVideoInfo(url.trim(), undefined, videoCookie)
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
    try { const file = await downloadVideo({ url: result.webpage_url || url.trim(), platform: result.platform, format: 'mp4', cookie: videoCookie }); triggerFileDownload(file); message.success('视频下载已开始') } catch (error) { message.error(error instanceof Error ? error.message : '视频下载失败') } finally { setDownloading(false) }
  }

  const openCookieDrawer = () => { setCookieDraft(videoCookie); setCookieDrawerOpen(true) }
  const saveCookie = () => {
    const value = cookieDraft.trim()
    setVideoCookie(value)
    try { window.localStorage.setItem(videoCookieStorageKey, value) } catch { message.warning('Cookie 已应用，但写入 localStorage 失败') }
    setCookieDrawerOpen(false)
    message.success(value ? '视频 Cookie 已保存' : '视频 Cookie 已清除')
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
        {loading ? <div className={styles.videoEmpty} aria-live="polite"><Spin size="large" /><strong>正在解析视频…</strong><span>请稍候，视频区域将在解析完成后更新。</span></div> : parseError ? <div className={`${styles.videoEmpty} ${styles.videoError}`} role="alert"><ExclamationCircleFilled /><strong>视频解析失败</strong><span>{parseError}</span></div> : result && result.platform === 'youtube' && result.id ? <iframe title={result.title || 'YouTube 视频预览'} src={`https://www.youtube.com/embed/${encodeURIComponent(result.id)}`} allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowFullScreen /> : result && result.platform === 'tiktok' && result.id ? <iframe title={result.title || 'TikTok 视频预览'} src={`https://www.tiktok.com/player/v1/${encodeURIComponent(result.id)}?lang=zh-CN`} allow="autoplay; fullscreen" allowFullScreen /> : result && (result.video_url && !previewError ? <video controls playsInline preload="metadata" poster={proxyURL(result.cover_url)} src={proxyURL(result.video_url)} onError={() => setPreviewError(true)}>您的浏览器不支持视频播放。</video> : <div className={styles.videoEmpty}><PlayCircleFilled /><strong>{result.video_url ? '当前视频地址暂不支持直接预览' : '当前结果没有可预览的视频地址'}</strong><span>可以尝试点击右上角下载视频。</span></div>)}
      </div>
    </section>}
    <section className={styles.platformSection}><div className={styles.sectionTitle}><span>SUPPORTED SOURCES</span><h2>支持的平台</h2></div><div className={styles.platformGrid}>{platforms.map((platform) => <div className={styles.platform} key={platform.name}><span className={styles.platformDot} style={{ background: platform.color }} />{platform.name}<span className={styles.platformArrow}>↗</span></div>)}</div></section>
    <aside className={styles.tip}><span>TIP</span><div><strong>链接解析</strong><p>支持完整网页链接和大部分平台的分享文案。下载前会保留原始画质选项。</p></div></aside>
    <Drawer title="视频 Cookie" placement="right" width={420} open={cookieDrawerOpen} onClose={() => setCookieDrawerOpen(false)}>
      <div className={styles.cookieDrawer}>
        <div className={styles.cookieNotice}><KeyOutlined /><span>所有视频平台共用这一份 Cookie。内容仅保存到当前浏览器，并通过请求 body 发送，不会拼接到 URL。</span></div>
        <label className={styles.cookieLabel} htmlFor="video-cookie-value">粘贴浏览器 Cookie</label>
        <Input.TextArea id="video-cookie-value" value={cookieDraft} onChange={(event) => setCookieDraft(event.target.value)} rows={8} placeholder="粘贴浏览器请求头中的完整 Cookie 字符串" spellCheck={false} />
        <button type="button" className={styles.cookieSaveButton} onClick={saveCookie}><KeyOutlined />保存视频 Cookie</button>
        <div className={styles.cookieGuide}><h3>如何获取</h3><ol><li>打开目标视频平台并确保页面可以正常访问。</li><li>在开发者工具 Network 面板刷新页面。</li><li>复制请求头中的完整 Cookie，不要只复制一个字段。</li></ol><p>抖音遇到 403 时，Cookie 通常需要包含 UIFID/UIFID_TEMP；Cookie 过期后重新获取即可。</p></div>
      </div>
    </Drawer>
    {captcha}
  </div>
}
