import { CheckCircleFilled, CloudDownloadOutlined, LinkOutlined, PlayCircleFilled, ThunderboltOutlined } from '@ant-design/icons'
import { message, Spin } from 'antd'
import { useState } from 'react'

import { downloadVideo, getVideoInfo, type VideoInfo } from '../../api/media'

import styles from './index.module.scss'

const platforms = [
  { name: 'Bilibili', color: '#00aeec' }, { name: '抖音', color: '#151515' },
  { name: '小红书', color: '#ff2442' }, { name: '微博', color: '#e6162d' },
  { name: '优酷', color: '#24b58a' }, { name: '爱奇艺', color: '#00be6e' },
]

function duration(seconds: number) {
  if (!seconds) return '未知时长'
  const minutes = Math.floor(seconds / 60)
  return `${minutes}分${Math.round(seconds % 60)}秒`
}

function triggerDownload(url: string) {
  const anchor = document.createElement('a')
  anchor.href = url; anchor.download = ''; anchor.target = '_blank'; anchor.rel = 'noreferrer'
  document.body.appendChild(anchor); anchor.click(); anchor.remove()
}

export default function VideoDownload() {
  const [url, setUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [downloading, setDownloading] = useState(false)
  const [previewError, setPreviewError] = useState(false)
  const [result, setResult] = useState<VideoInfo | null>(null)

  const parse = async () => {
    if (!url.trim()) { message.warning('请先粘贴视频链接'); return }
    setLoading(true)
    try { setPreviewError(false); setResult(await getVideoInfo(url.trim())); message.success('视频解析完成') } catch (error) { setResult(null); message.error(error instanceof Error ? error.message : '视频解析失败') } finally { setLoading(false) }
  }

  const download = async () => {
    if (!result) return
    setDownloading(true)
    try { const file = await downloadVideo({ url: result.webpage_url || url.trim(), platform: result.platform, format: 'mp4', cover: true }); if (file.file_url) triggerDownload(file.file_url); message.success('视频下载已开始') } catch (error) { message.error(error instanceof Error ? error.message : '视频下载失败') } finally { setDownloading(false) }
  }

  return <div className={styles.page}>
    <div className={styles.pageIntro}><div className={styles.overline}>VIDEO / 01</div><h1>视频下载</h1><p>复制分享链接，提取你想要的清晰度。</p></div>
    <section className={styles.workspace}>
      <div className={styles.panelHead}><div><span className={styles.panelEyebrow}>QUICK EXTRACT</span><h2>粘贴链接开始</h2></div><span className={styles.step}>STEP 01 <b>→</b> 02</span></div>
      <div className={styles.inputRow}><div className={styles.inputWrap}><LinkOutlined /><input value={url} onChange={(event) => setUrl(event.target.value)} onKeyDown={(event) => event.key === 'Enter' && parse()} placeholder="粘贴 Bilibili、抖音、小红书等视频链接" aria-label="视频链接" /><button type="button" aria-label="清空链接" onClick={() => { setUrl(''); setResult(null); setPreviewError(false) }}>×</button></div><button type="button" className={styles.parseButton} onClick={parse} disabled={loading}>{loading ? <><Spin size="small" />解析中...</> : <><ThunderboltOutlined />提取视频</>}</button></div>
      <div className={styles.hint}><CheckCircleFilled />支持分享口令自动识别 · 解析结果仅在本地展示</div>
    </section>
    {result && <section className={styles.previewSection} aria-labelledby="video-preview-title">
      <div className={styles.previewHeader}>
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
      </div>
      <div className={styles.videoStage}>
        {result.video_url && !previewError ? <video controls playsInline preload="metadata" poster={result.cover_url || undefined} src={result.video_url} onError={() => setPreviewError(true)}>您的浏览器不支持视频播放。</video> : <div className={styles.videoEmpty}><PlayCircleFilled /><strong>{result.video_url ? '当前视频地址暂不支持直接预览' : '当前结果没有可预览的视频地址'}</strong><span>可以尝试点击右上角下载视频。</span></div>}
      </div>
    </section>}
    <section className={styles.platformSection}><div className={styles.sectionTitle}><span>SUPPORTED SOURCES</span><h2>支持的平台</h2></div><div className={styles.platformGrid}>{platforms.map((platform) => <div className={styles.platform} key={platform.name}><span className={styles.platformDot} style={{ background: platform.color }} />{platform.name}<span className={styles.platformArrow}>↗</span></div>)}</div></section>
    <aside className={styles.tip}><span>TIP</span><div><strong>链接解析</strong><p>支持完整网页链接和大部分平台的分享文案。下载前会保留原始画质选项。</p></div></aside>
  </div>
}
