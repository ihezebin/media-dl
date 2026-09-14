import {
  AppleFilled, BilibiliFilled, ControlOutlined, CustomerServiceOutlined, DeleteOutlined, DownloadOutlined, FileTextOutlined,
  FolderOpenOutlined, GlobalOutlined, KeyOutlined, LinkOutlined, PictureOutlined, PlusOutlined, QqOutlined,
  PlayCircleFilled, PauseCircleFilled, SearchOutlined, SoundOutlined,
  StepBackwardOutlined, StepForwardOutlined, CloseOutlined, UserOutlined,
} from '@ant-design/icons'
import { Drawer, Input, message, Select, Spin, Tooltip } from 'antd'
import { useEffect, useMemo, useRef, useState } from 'react'
import type { CSSProperties } from 'react'

import { downloadMusicAsset, getMusicLyrics, getMusicPlatforms, resolveMusicSong, searchMusic, type MusicSong } from '../../api/media'

import styles from './index.module.scss'

const defaultPlatforms = ['netease', 'qq', 'kugou', 'kuwo', 'migu', 'fivesing', 'qianqian', 'soda', 'jamendo', 'joox', 'bilibili', 'apple']
const cookieStorageKey = 'media-dl.music.platform-cookies'
const platformLabels: Record<string, string> = { netease: '网易云', qq: 'QQ音乐', kugou: '酷狗', kuwo: '酷我', migu: '咪咕', fivesing: '5sing', qianqian: '千千', soda: '汽水', jamendo: 'Jamendo', joox: 'JOOX', bilibili: 'Bilibili', apple: 'Apple' }
const platformColors: Record<string, string> = { netease: '#e43f4f', qq: '#22c875', kugou: '#2f7df6', kuwo: '#5967d8', migu: '#11b8d1', fivesing: '#ef5c98', qianqian: '#3b82f6', soda: '#f59e0b', jamendo: '#2fa85f', joox: '#f06a35', bilibili: '#00aeec', apple: '#5f6675' }
const searchTypeMeta = [
  { value: 'song' as const, label: '单曲', hint: '精准定位歌曲', color: '#1677ff' },
  { value: 'artist' as const, label: '歌手', hint: '查看艺人作品', color: '#8b5cf6' },
  { value: 'album' as const, label: '专辑', hint: '探索完整专辑', color: '#f97316' },
]

const cookieGuides: Record<string, { site: string; steps: string[] }> = {
  netease: { site: 'music.163.com', steps: ['打开网易云音乐网页版并登录。', '按 ⌥⌘I（Windows/Linux 为 F12）打开开发者工具，进入“网络 / Network”。', '刷新页面或播放一首歌，在请求的 Request Headers 中找到 Cookie，复制完整内容。'] },
  qq: { site: 'y.qq.com', steps: ['打开 QQ 音乐网页版并登录。', '打开开发者工具的“网络 / Network”，刷新或搜索一首歌。', '选择请求，在 Request Headers 中复制完整的 Cookie 值。'] },
  kugou: { site: 'kugou.com', steps: ['打开酷狗网页版并登录。', '打开开发者工具的“网络 / Network”，刷新歌曲页面。', '选择 kugou.com 或音频请求，在 Request Headers 中复制 Cookie。'] },
  kuwo: { site: 'kuwo.cn', steps: ['打开酷我音乐网页版并登录。', '打开开发者工具的“网络 / Network”，刷新或播放歌曲。', '从请求头 Request Headers 复制完整 Cookie。'] },
  migu: { site: 'music.migu.cn', steps: ['打开咪咕音乐网页版并登录。', '打开开发者工具的“网络 / Network”，刷新歌曲页面。', '找到请求头中的 Cookie 并完整复制。'] },
  fivesing: { site: '5sing.kugou.com', steps: ['打开 5sing 网页并登录。', '打开开发者工具的“网络 / Network”，刷新或打开歌曲。', '从请求头复制完整 Cookie。'] },
  qianqian: { site: 'qianqian.com', steps: ['打开千千音乐网页版并登录。', '打开开发者工具的“网络 / Network”，刷新或搜索歌曲。', '从请求头复制完整 Cookie。'] },
  soda: { site: 'qishui.douyin.com', steps: ['打开汽水音乐网页版并登录。', '打开开发者工具的“网络 / Network”，刷新或播放歌曲。', '从相关请求的 Request Headers 复制完整 Cookie。'] },
  jamendo: { site: 'jamendo.com', steps: ['打开 Jamendo 网站并登录（如果该资源需要登录）。', '打开开发者工具的“网络 / Network”，刷新页面。', '从请求头复制完整 Cookie。'] },
  joox: { site: 'joox.com', steps: ['打开 JOOX 网页并登录。', '打开开发者工具的“网络 / Network”，刷新或播放歌曲。', '从请求头复制完整 Cookie。'] },
  bilibili: { site: 'bilibili.com', steps: ['打开哔哩哔哩并登录。', '打开开发者工具的“网络 / Network”，刷新页面或播放相关内容。', '选择 bilibili.com 请求，复制 Request Headers 中的 Cookie。'] },
  apple: { site: 'music.apple.com', steps: ['打开 Apple Music 网页并登录。', '打开开发者工具的“网络 / Network”，刷新歌曲页面。', '从请求头复制完整 Cookie；部分资源还可能需要账户授权。'] },
}

function SearchTypeIcon({ type }: { type: typeof searchTypeMeta[number]['value'] }) {
  if (type === 'artist') return <UserOutlined />
  if (type === 'album') return <FolderOpenOutlined />
  return <SoundOutlined />
}

function PlatformLogo({ platform }: { platform: string }) {
  if (platform === 'qq') return <QqOutlined />
  if (platform === 'bilibili') return <BilibiliFilled />
  if (platform === 'apple') return <AppleFilled />
  if (platform === 'netease') return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M19.2 11.2a5.8 5.8 0 0 0-11-2.4A4.8 4.8 0 0 0 8.4 18h10.2a3.4 3.4 0 0 0 .6-6.8Zm-9.8 2.1c2.1-1.4 4.6-1.7 7.4-.8-2.5.1-4.4.8-5.7 2.1-.5.5-1.2.5-1.7.1-.5-.4-.5-1 0-1.4Zm.8 2.3c1.7-1.1 3.5-1.4 5.5-.9-1.6.3-2.8.8-3.6 1.5-.6.5-1.3.5-1.8.1-.5-.2-.5-.5-.1-.7Z" /></svg>
  if (platform === 'kugou') return <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="7" cy="7" r="3" /><circle cx="17" cy="7" r="3" /><circle cx="5.5" cy="13" r="2.6" /><circle cx="18.5" cy="13" r="2.6" /><path d="M12 10.5c-3.6 0-5.7 2.5-5.7 5.1 0 2.2 1.7 3.4 3.4 3.4 1 0 1.7-.5 2.3-1.1.6.6 1.3 1.1 2.3 1.1 1.7 0 3.4-1.2 3.4-3.4 0-2.6-2.1-5.1-5.7-5.1Z" /></svg>
  if (platform === 'kuwo') return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 5.5h3.1l3.3 5.2 3.3-5.2H18l-4.9 7.1V19h-3v-6.4L5 5.5Zm11.4 8.2h2.5V19h-2.5v-5.3Z" /></svg>
  if (platform === 'migu') return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5.2 14.4a4.2 4.2 0 1 1 3.3-6.8 5.7 5.7 0 0 1 10.3 3.3 3.8 3.8 0 0 1-.8 7.5H8.1a3.4 3.4 0 0 1-2.9-4Z" /><circle cx="15.8" cy="7" r="1.5" fill="currentColor" opacity=".8" /></svg>
  if (platform === 'fivesing') return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 5h12v3H9v2.2h4.3c3.1 0 5 1.6 5 4.3 0 2.8-2.1 4.5-5.6 4.5H6v-3h6.4c1.5 0 2.5-.5 2.5-1.5s-.9-1.4-2.5-1.4H6V5Z" /></svg>
  if (platform === 'qianqian') return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 5h12v2.6H6V5Zm0 5.2h12v2.6H6v-2.6Zm0 5.2h12V18H6v-2.6Z" /><circle cx="4.5" cy="6.3" r="1.2" /><circle cx="4.5" cy="11.5" r="1.2" /><circle cx="4.5" cy="16.7" r="1.2" /></svg>
  if (platform === 'soda') return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 6h12v3H9v2h4.7c3 0 4.8 1.5 4.8 4s-2 4-5.4 4H6v-3h6.7c1.5 0 2.4-.3 2.4-1.1 0-.7-.8-1-2.4-1H6V6Z" /><circle cx="19" cy="5" r="1.3" /></svg>
  if (platform === 'jamendo') return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3.5a8.5 8.5 0 1 0 8.5 8.5A8.51 8.51 0 0 0 12 3.5Zm3.4 12.2H9.7a1.3 1.3 0 0 1 0-2.6h5.7a1.3 1.3 0 0 1 0 2.6Zm0-4.8H9.7a1.3 1.3 0 0 1 0-2.6h5.7a1.3 1.3 0 0 1 0 2.6Z" /></svg>
  if (platform === 'joox') return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 6.2a2.2 2.2 0 1 1 4.4 0v7.1a2.6 2.6 0 1 0 5.2 0V6.2a2.2 2.2 0 1 1 4.4 0v7.1a7 7 0 1 1-14 0V6.2Z" /></svg>
  return <GlobalOutlined />
}

function formatDuration(seconds: number) {
  if (!seconds) return '--:--'
  return `${String(Math.floor(seconds / 60)).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`
}

function Artwork({ song, large = false }: { song: MusicSong; large?: boolean }) {
  const fallback = platformColors[song.source] || '#1677ff'
  return <div className={large ? styles.artworkLarge : styles.artwork} style={{ background: song.cover ? undefined : `linear-gradient(145deg, ${fallback}, #121a32)` }}>{song.cover ? <img src={song.cover} alt="" /> : <SoundOutlined />}</div>
}

function parseLyrics(raw: string) {
  return raw.split(/\r?\n/).map((line) => line.replace(/^\s*\[\d{1,3}:\d{1,2}(?:\.\d{1,3})?\]\s*/, '').trim()).filter(Boolean)
}

function Player({ song, playing, currentTime, duration, lyrics, onClose, onToggle, onSeek }: { song: MusicSong; playing: boolean; currentTime: number; duration: number; lyrics: string[]; onClose: () => void; onToggle: () => void; onSeek: (value: number) => void }) {
  return <div className={styles.playerOverlay}>
    <div className={styles.playerTop}><span className={styles.playerMode}>NOW PLAYING / {platformLabels[song.source] || song.source}</span><button type="button" className={styles.closePlayer} onClick={onClose} aria-label="收起播放器"><CloseOutlined /></button></div>
    <div className={styles.playerBody}>
      <div className={styles.discSide}><div className={styles.discOrbit} /><div className={`${styles.disc} ${playing ? '' : styles.discPaused}`}><div className={styles.discGrooves} /><div className={styles.discCenter}><Artwork song={song} large /></div></div><div className={styles.tonearm}><span /></div></div>
      <div className={styles.lyricSide}><div className={styles.playerMeta}><span>来自 {song.album || '未知专辑'}</span><h2>{song.name}</h2><p>{song.artist || '未知歌手'}</p></div><div className={styles.lyrics}>{lyrics.length ? lyrics.map((line, index) => <p key={`${line}-${index}`} className={index === Math.min(2, lyrics.length - 1) ? styles.activeLyric : ''}>{line}</p>) : <p className={styles.lyricsLoading}>歌词加载中，或该歌曲暂无歌词</p>}</div></div>
    </div>
    <div className={styles.playerControls}><input className={styles.progressInput} type="range" min={0} max={duration || 1} step="any" value={Math.min(currentTime, duration || 1)} onChange={(event) => onSeek(Number(event.target.value))} aria-label="播放进度" /><div className={styles.controlRow}><time>{formatDuration(currentTime)}</time><div className={styles.controlButtons}><button type="button" aria-label="上一首"><StepBackwardOutlined /></button><button type="button" className={styles.playButton} aria-label={playing ? '暂停' : '播放'} onClick={onToggle}>{playing ? <PauseCircleFilled /> : <PlayCircleFilled />}</button><button type="button" aria-label="下一首"><StepForwardOutlined /></button></div><time>{formatDuration(duration || song.duration)}</time></div></div>
  </div>
}

function triggerDownload(url: string) {
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = ''
  anchor.target = '_blank'
  anchor.rel = 'noreferrer'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
}

function readStoredPlatformCookies() {
  if (typeof window === 'undefined') return {}
  try {
    const stored = JSON.parse(window.localStorage.getItem(cookieStorageKey) || '{}') as unknown
    if (!stored || typeof stored !== 'object' || Array.isArray(stored)) return {}
    return Object.fromEntries(Object.entries(stored).filter(([, value]) => typeof value === 'string' && value.trim()))
  } catch {
    return {}
  }
}

export default function MusicSearch() {
  const [keyword, setKeyword] = useState('')
  const [type, setType] = useState<'song' | 'artist' | 'album'>('song')
  const [platforms, setPlatforms] = useState(defaultPlatforms)
  const [selectedPlatforms, setSelectedPlatforms] = useState(defaultPlatforms)
  const [platformCookies, setPlatformCookies] = useState<Record<string, string>>(readStoredPlatformCookies)
  const [cookieDrafts, setCookieDrafts] = useState<Record<string, string>>({})
  const [cookiePlatform, setCookiePlatform] = useState('')
  const [cookieDraft, setCookieDraft] = useState('')
  const [cookieAddOpen, setCookieAddOpen] = useState(false)
  const [cookieDrawerOpen, setCookieDrawerOpen] = useState(false)
  const [songs, setSongs] = useState<MusicSong[]>([])
  const [searched, setSearched] = useState(false)
  const [loading, setLoading] = useState(false)
  const [playing, setPlaying] = useState<MusicSong | null>(null)
  const [isPlaying, setIsPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [lyrics, setLyrics] = useState<string[]>([])
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const platformCookiesRef = useRef<Record<string, string>>({})

  useEffect(() => { getMusicPlatforms().then((result) => { if (result.platforms.length) { setPlatforms(result.platforms); setSelectedPlatforms(result.platforms) } }).catch(() => undefined) }, [])
  useEffect(() => { platformCookiesRef.current = platformCookies }, [platformCookies])
  useEffect(() => () => { audioRef.current?.pause(); audioRef.current = null }, [])

  useEffect(() => {
    const song = playing
    audioRef.current?.pause()
    audioRef.current = null
    setIsPlaying(false); setCurrentTime(0); setDuration(0); setLyrics([])
    if (!song) return
    let cancelled = false
    const audio = new Audio(song.url)
    audioRef.current = audio
    const onTime = () => setCurrentTime(audio.currentTime)
    const onMeta = () => setDuration(audio.duration || song.duration || 0)
    const onEnded = () => setIsPlaying(false)
    const onError = () => message.error('播放地址不可用，请尝试下载歌曲')
    audio.addEventListener('timeupdate', onTime); audio.addEventListener('loadedmetadata', onMeta); audio.addEventListener('ended', onEnded); audio.addEventListener('error', onError)
    if (song.url) audio.play().then(() => !cancelled && setIsPlaying(true)).catch(() => !cancelled && setIsPlaying(false))
    getMusicLyrics(song, platformCookiesRef.current[song.source] || '').then((result) => !cancelled && setLyrics(parseLyrics(result.lyrics))).catch(() => !cancelled && setLyrics([]))
    return () => { cancelled = true; audio.pause(); audio.removeEventListener('timeupdate', onTime); audio.removeEventListener('loadedmetadata', onMeta); audio.removeEventListener('ended', onEnded); audio.removeEventListener('error', onError) }
  }, [playing])

  const allSelected = selectedPlatforms.length === platforms.length
  const selectedCount = selectedPlatforms.length
  const invalidCount = useMemo(() => songs.filter((song) => song.is_invalid).length, [songs])
  const resultHint = useMemo(() => searched ? `${songs.length} 首结果 · ${selectedCount} 个平台${invalidCount ? ` · ${invalidCount} 首无效` : ''}` : '默认展示所有平台', [searched, selectedCount, songs.length, invalidCount])

  const search = async () => {
    if (!keyword.trim()) { message.warning('请输入歌曲名、歌手或专辑'); return }
    if (!selectedPlatforms.length) { message.warning('请至少选择一个音乐平台'); return }
    setLoading(true)
    try {
      const result = await searchMusic({ keyword: keyword.trim(), type, platforms: selectedPlatforms, cookies: platformCookies })
      setSongs(result.results); setSearched(true)
      if (result.errors?.length) message.warning(`${result.errors.length} 个平台搜索失败，已展示可用结果`)
    } catch (error) { message.error(error instanceof Error ? error.message : '音乐搜索失败'); setSongs([]); setSearched(true) } finally { setLoading(false) }
  }

  const play = async (song: MusicSong) => {
    if (song.is_invalid) { message.warning('该歌曲已标记为无效，无法播放'); return }
    let playable = song
    if (!song.url && song.link) {
      try { playable = await resolveMusicSong({ source: song.source, link: song.link }, platformCookies[song.source] || '') } catch (error) { message.error(error instanceof Error ? error.message : '无法解析播放地址'); return }
    }
    if (!playable.url) { message.error('该歌曲没有可用的播放地址'); return }
    setPlaying(playable)
  }

  const togglePlaying = () => {
    const audio = audioRef.current
    if (!audio) return
    if (audio.paused) audio.play().then(() => setIsPlaying(true)).catch(() => message.error('播放失败'))
    else { audio.pause(); setIsPlaying(false) }
  }

  const download = async (song: MusicSong, action: 'audio' | 'cover' | 'lyrics') => {
    if (song.is_invalid && action === 'audio') { message.warning('该歌曲已标记为无效，无法下载音频'); return }
    try { const result = await downloadMusicAsset(song, action, platformCookies[song.source] || ''); if (result.file_url) triggerDownload(result.file_url); message.success(`${action === 'audio' ? '歌曲' : action === 'cover' ? '封面' : '歌词'}下载已开始`) } catch (error) { message.error(error instanceof Error ? error.message : '下载失败') }
  }

  const togglePlatform = (platform: string) => setSelectedPlatforms((current) => current.includes(platform) ? current.filter((item) => item !== platform) : [...current, platform])
  const openCookieDrawer = () => { setCookieDrafts({ ...platformCookies }); setCookiePlatform(''); setCookieDraft(''); setCookieAddOpen(false); setCookieDrawerOpen(true) }
  const changeCookiePlatform = (platform: string) => { setCookiePlatform(platform); setCookieDraft('') }
  const addCookie = () => { const value = cookieDraft.trim(); if (!cookiePlatform) { message.warning('请先选择平台'); return } if (!value) { message.warning('请粘贴 Cookie 后再添加'); return } setCookieDrafts((current) => ({ ...current, [cookiePlatform]: value })); setCookiePlatform(''); setCookieDraft(''); setCookieAddOpen(false); message.success(`已添加${platformLabels[cookiePlatform] || cookiePlatform} Cookie`) }
  const updateCookie = (platform: string, value: string) => setCookieDrafts((current) => ({ ...current, [platform]: value }))
  const removeCookie = (platform: string) => setCookieDrafts((current) => { const next = { ...current }; delete next[platform]; return next })
  const saveCookies = () => { const next = Object.fromEntries(Object.entries(cookieDrafts).map(([platform, value]) => [platform, value.trim()]).filter(([, value]) => value)) as Record<string, string>; setPlatformCookies(next); try { window.localStorage.setItem(cookieStorageKey, JSON.stringify(next)) } catch { message.warning('Cookie 已应用，但写入 localStorage 失败') } setCookieDrawerOpen(false); message.success(`已保存 ${Object.keys(next).length} 个平台 Cookie`) }
  const cookieEntries = Object.entries(cookieDrafts).filter(([, value]) => value.trim())
  const availableCookiePlatforms = platforms.filter((platform) => !cookieDrafts[platform]?.trim())
  const cookieGuide = cookieGuides[cookiePlatform] || { site: platformLabels[cookiePlatform] || cookiePlatform, steps: ['打开对应平台网站并登录。', '在开发者工具的 Network 面板中刷新或播放歌曲。', '从请求头 Request Headers 复制完整 Cookie。'] }

  return <div className={styles.page}>
    <div className={styles.pageIntro}><div className={styles.pageIntroRow}><div><div className={styles.overline}>MUSIC / 02</div><h1>搜索音乐</h1></div><button type="button" className={styles.cookieButton} onClick={openCookieDrawer}><KeyOutlined />平台 Cookie</button></div><p>在多个平台找到同一首歌，选择你喜欢的版本。</p></div>
    <section className={styles.searchPanel}>
      <div className={styles.searchRow}><div className={styles.searchInput}><SearchOutlined /><input value={keyword} onChange={(event) => setKeyword(event.target.value)} onKeyDown={(event) => event.key === 'Enter' && search()} placeholder="搜索歌曲名、歌手或专辑" aria-label="搜索音乐" /></div><button type="button" className={styles.searchButton} onClick={search} disabled={loading}><SearchOutlined />{loading ? '搜索中' : '搜索'}</button></div>
      <div className={styles.searchTypeBlock}>
        <span className={styles.searchTypeLabel}><ControlOutlined />搜索类型</span>
        <div className={styles.searchTypes} role="radiogroup" aria-label="搜索类型">
          {searchTypeMeta.map((item) => (
            <button
              key={item.value}
              type="button"
              role="radio"
              aria-checked={type === item.value}
              className={`${styles.searchTypeButton} ${type === item.value ? styles.searchTypeButtonActive : ''}`}
              style={{ '--choice-color': item.color } as CSSProperties}
              onClick={() => setType(item.value)}>
              <span className={styles.searchTypeIcon}><SearchTypeIcon type={item.value} /></span>
              <span className={styles.searchTypeCopy}><strong>{item.label}</strong><small>{item.hint}</small></span>
              <span className={styles.searchTypeCheck}>{type === item.value ? '✓' : ''}</span>
            </button>
          ))}
        </div>
      </div>
      <div className={styles.platformHead}><div className={styles.platformTitle}><span className={styles.platformTitleIcon}><ControlOutlined /></span><span>平台筛选</span><em>{selectedCount}/{platforms.length} 已选</em></div><button type="button" onClick={() => setSelectedPlatforms(allSelected ? [] : platforms)}>{allSelected ? '取消全选' : '全选平台'}</button></div>
      <div className={styles.platforms}>{platforms.map((platform) => <button type="button" key={platform} aria-pressed={selectedPlatforms.includes(platform)} className={`${styles.platformChip} ${selectedPlatforms.includes(platform) ? styles.platformChipActive : ''}`} style={{ '--platform-color': platformColors[platform] || 'var(--app-accent)' } as CSSProperties} onClick={() => togglePlatform(platform)}><span className={styles.platformIcon}><PlatformLogo platform={platform} /></span><span className={styles.chipLabel}>{platformLabels[platform] || platform}</span><span className={styles.chipMark}>{selectedPlatforms.includes(platform) ? '✓' : ''}</span></button>)}</div>
    </section>
    <section className={styles.results}><div className={styles.resultsHead}><div><span className={styles.resultEyebrow}>RESULTS / {songs.length.toString().padStart(2, '0')}</span><h2>{searched ? '搜索结果' : '等待搜索'}</h2></div><span className={styles.resultHint}>{resultHint}</span></div>{loading ? <div className={styles.empty}><Spin /><p>正在请求音乐平台</p></div> : songs.length ? <div className={styles.trackList}>{songs.map((song, index) => <article className={`${styles.track} ${song.is_invalid ? styles.trackInvalid : ''}`} key={`${song.source}-${song.id}-${index}`}><span className={styles.trackNo}>{String(index + 1).padStart(2, '0')}</span><button type="button" className={styles.trackCoverButton} aria-label={`播放 ${song.name}`} onClick={() => play(song)} disabled={song.is_invalid}><Artwork song={song} /><span className={styles.coverPlay}><PlayCircleFilled /></span></button><div className={styles.trackInfo}><h3><span className={styles.trackTitle}>{song.name}</span>{song.is_invalid && <span className={styles.invalidBadge}>无效</span>}</h3><p>{song.artist || '未知歌手'} <span>·</span> {song.album || '未知专辑'}</p></div><span className={styles.source} style={{ '--source-color': platformColors[song.source] || 'var(--app-accent)' } as CSSProperties}>{platformLabels[song.source] || song.source}</span><span className={styles.duration}>{formatDuration(song.duration)}</span><div className={styles.trackActions}><Tooltip title={song.is_invalid ? '歌曲无效' : '播放'}><button type="button" onClick={() => play(song)} aria-label="播放" disabled={song.is_invalid}><PlayCircleFilled /></button></Tooltip><Tooltip title={song.is_invalid ? '歌曲无效' : '下载歌曲'}><button type="button" onClick={() => download(song, 'audio')} aria-label="下载歌曲" disabled={song.is_invalid}><DownloadOutlined /></button></Tooltip><Tooltip title="下载歌词"><button type="button" onClick={() => download(song, 'lyrics')} aria-label="下载歌词"><FileTextOutlined /></button></Tooltip><Tooltip title="下载封面"><button type="button" onClick={() => download(song, 'cover')} aria-label="下载封面"><PictureOutlined /></button></Tooltip></div></article>)}</div> : <div className={styles.empty}><CustomerServiceOutlined /><p>{searched ? '没有找到匹配的曲目' : '输入关键词开始跨平台搜索'}</p><span>{searched ? '试试调整关键词或平台筛选' : '歌曲、歌手和专辑均可作为搜索类型'}</span></div>}</section>
    <Drawer title="平台 Cookie" placement="right" width={420} open={cookieDrawerOpen} onClose={() => setCookieDrawerOpen(false)}>
      <div className={styles.cookieDrawer}>
        <div className={styles.cookieNotice}><KeyOutlined /><span>可配置多个平台。点击“保存全部 Cookie”后写入当前浏览器 localStorage，并通过请求 body 发送，不会拼接到 URL。</span></div>
        <div className={styles.cookieSectionHead}><strong>已添加的平台</strong><span>{cookieEntries.length} 个</span></div>
        {cookieEntries.length ? <div className={styles.cookieEntries}>{cookieEntries.map(([platform, value]) => <div className={styles.cookieEntry} key={platform}>
          <div className={styles.cookieEntryHead}><div className={styles.cookiePlatform}><span className={styles.cookiePlatformIcon} style={{ '--platform-color': platformColors[platform] || 'var(--app-accent)' } as CSSProperties}><PlatformLogo platform={platform} /></span><div><strong>{platformLabels[platform] || platform}</strong><small>{cookieGuides[platform]?.site || platform}</small></div></div><button type="button" className={styles.cookieRemoveButton} onClick={() => removeCookie(platform)} aria-label={`移除${platformLabels[platform] || platform} Cookie`}><DeleteOutlined /></button></div>
          <Input.TextArea value={value} onChange={(event) => updateCookie(platform, event.target.value)} rows={4} placeholder="粘贴浏览器请求头中的完整 Cookie 字符串" spellCheck={false} />
        </div>)}</div> : <div className={styles.cookieEmpty}>还没有添加平台 Cookie</div>}
        {availableCookiePlatforms.length ? <div className={styles.cookieAddArea}>
          <button type="button" className={styles.cookieAddButton} onClick={() => setCookieAddOpen((open) => !open)}><PlusOutlined />添加平台 Cookie</button>
          {cookieAddOpen && <div className={styles.cookieAddForm}>
            <label className={styles.cookieLabel} htmlFor="cookie-platform">选择未添加的平台</label>
            <Select id="cookie-platform" className={styles.cookieSelect} placeholder="选择平台" value={cookiePlatform || undefined} onChange={changeCookiePlatform} options={availableCookiePlatforms.map((platform) => ({ value: platform, label: platformLabels[platform] || platform }))} />
            {cookiePlatform && <><div className={styles.cookiePlatform}><span className={styles.cookiePlatformIcon} style={{ '--platform-color': platformColors[cookiePlatform] || 'var(--app-accent)' } as CSSProperties}><PlatformLogo platform={cookiePlatform} /></span><div><strong>{platformLabels[cookiePlatform] || cookiePlatform}</strong><small>{cookieGuide.site}</small></div></div><label className={styles.cookieLabel} htmlFor="platform-cookie-value">粘贴 Cookie</label><Input.TextArea id="platform-cookie-value" value={cookieDraft} onChange={(event) => setCookieDraft(event.target.value)} rows={6} placeholder="粘贴浏览器请求头中的完整 Cookie 字符串" spellCheck={false} /><button type="button" className={styles.cookieAddConfirm} onClick={addCookie}><PlusOutlined />添加到待保存列表</button><div className={styles.cookieGuide}><h3><LinkOutlined />如何获取并复制</h3><ol>{cookieGuide.steps.map((step) => <li key={step}>{step}</li>)}</ol><p>请复制完整的 Cookie 请求头内容，不要只复制某一个字段。Cookie 过期后重新获取即可。</p></div></>}
          </div>}
        </div> : <div className={styles.cookieAllAdded}>已添加全部可用平台</div>}
        <button type="button" className={styles.cookieSaveButton} onClick={saveCookies}><KeyOutlined />保存全部 Cookie</button>
      </div>
    </Drawer>
    {playing && <Player song={playing} playing={isPlaying} currentTime={currentTime} duration={duration} lyrics={lyrics} onClose={() => setPlaying(null)} onToggle={togglePlaying} onSeek={(value) => { if (audioRef.current) audioRef.current.currentTime = value; setCurrentTime(value) }} />}
  </div>
}
