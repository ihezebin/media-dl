export type MusicSong = {
  id: string
  name: string
  artist: string
  album: string
  duration: number
  source: string
  url: string
  ext: string
  cover: string
  link: string
  is_vip?: boolean
  is_invalid?: boolean
  extra?: Record<string, string>
}

export type MusicSearchResponse = {
  keyword: string
  platforms: string[]
  results: MusicSong[]
  errors?: { platform: string; error: string }[]
}

export type VideoInfo = {
  success: boolean
  platform: string
  id: string
  title: string
  description: string
  author: string
  duration: number
  cover_url: string
  webpage_url: string
  video_url: string
  formats: { format_id: string; quality: string; ext: string; width: number; height: number; has_audio: boolean; has_video: boolean }[]
}

export type DownloadedFile = { blob: Blob; filename: string }

type Envelope<T> = { code: number; message?: string; data: T }

export class APIError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'APIError'
    this.status = status
  }
}

const proxyHeaderNames: Record<string, string> = { Referer: 'referer', Origin: 'origin', 'User-Agent': 'user_agent' }

// API responses keep upstream URLs; only browser media requests use /api/proxy.
export function proxyURL(raw: string, headers?: Record<string, string>) {
  const value = raw.trim()
  if (!value) return raw
  try {
    const parsed = new URL(value, window.location.origin)
    if (parsed.origin === window.location.origin && parsed.pathname === '/api/proxy' && parsed.searchParams.has('url')) return value
  } catch {
    return raw
  }
  if (!/^https?:\/\//i.test(value)) return raw
  const query = new URLSearchParams({ url: value })
  Object.entries(proxyHeaderNames).forEach(([headerName, queryName]) => {
    const headerValue = headers?.[headerName]?.trim()
    if (headerValue) query.set(queryName, headerValue)
  })
  return `/api/proxy?${query.toString()}`
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
  })
  const payload = (await response.json().catch(() => null)) as Envelope<T> | null
  if (!response.ok || !payload || payload.code !== 0) {
    throw new APIError(payload?.message || `请求失败 (${response.status})`, response.status)
  }
  return payload.data
}

function downloadFilename(response: Response) {
  const disposition = response.headers.get('Content-Disposition') || ''
  const encoded = disposition.match(/filename\*=UTF-8''([^;]+)/i)?.[1]
  if (encoded) {
    try { return decodeURIComponent(encoded.replace(/\+/g, ' ')) } catch { /* fall back to filename */ }
  }
  const fallback = disposition.match(/filename="([^"]+)"/i)?.[1] || disposition.match(/filename=([^;]+)/i)?.[1]
  return fallback?.trim() || 'download'
}

async function downloadRequest(path: string, init?: RequestInit): Promise<DownloadedFile> {
  const response = await fetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
  })
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as Envelope<unknown> | null
    throw new APIError(payload?.message || `请求失败 (${response.status})`, response.status)
  }
  return { blob: await response.blob(), filename: downloadFilename(response) }
}

export function triggerFileDownload(file: DownloadedFile) {
  const objectURL = URL.createObjectURL(file.blob)
  const anchor = document.createElement('a')
  anchor.href = objectURL
  anchor.download = file.filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000)
}

export type BehaviorCaptcha = {
  id: string
  type: 'slide' | 'drag' | 'rotate'
  expires_at: string
  image: string
  thumb: string
  thumb_size?: number
  thumb_x?: number
  thumb_y?: number
  thumb_width?: number
  thumb_height?: number
  angle?: number
}

let behaviorCaptchaToken: string | null = null
let behaviorCaptchaExpiresAt = 0

export function getBehaviorCaptcha() {
  return request<BehaviorCaptcha>('/api/captcha')
}

export function verifyBehaviorCaptcha(payload: { id: string; type: BehaviorCaptcha['type']; x?: number; y?: number; angle?: number }) {
  return request<{ verified: boolean; token: string; expires_at: string }>('/api/captcha/verify', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function setBehaviorCaptchaToken(token: string, expiresAt: string) {
  const timestamp = Date.parse(expiresAt)
  if (!token || !Number.isFinite(timestamp)) return
  behaviorCaptchaToken = token
  behaviorCaptchaExpiresAt = timestamp
}

export function forgetBehaviorCaptcha() {
  behaviorCaptchaToken = null
  behaviorCaptchaExpiresAt = 0
}

export function hasBehaviorCaptchaToken() {
  if (!behaviorCaptchaToken || behaviorCaptchaExpiresAt <= Date.now()) {
    forgetBehaviorCaptcha()
    return false
  }
  return true
}

function behaviorCaptchaHeaders(): HeadersInit {
  return behaviorCaptchaToken ? { 'X-Captcha-Token': behaviorCaptchaToken } : {}
}

export function isCaptchaRequired(error: unknown) {
  return error instanceof APIError && error.status === 401
}

export function getMusicPlatforms() {
  return request<{ platforms: string[] }>('/api/music/platforms')
}

export function searchMusic(params: { keyword: string; type: 'song' | 'artist' | 'album'; platforms: string[]; limit?: number; cookies?: Record<string, string> }) {
  return request<MusicSearchResponse>('/api/music/search/verified', {
    method: 'POST',
    headers: behaviorCaptchaHeaders(),
    body: JSON.stringify({
      keyword: params.keyword,
      type: params.type,
      platforms: params.platforms,
      limit: params.limit ?? 10,
      cookies: params.cookies ?? {},
    }),
  })
}

export function resolveMusicSong(song: Pick<MusicSong, 'source' | 'link'>, cookie = '') {
  return request<MusicSong>('/api/music/resolve', {
    method: 'POST',
    body: JSON.stringify({ platform: song.source, url: song.link, cookie }),
  })
}

export function getMusicLyrics(song: MusicSong, cookie = '') {
  return request<{ lyrics: string }>('/api/music/lyrics', {
    method: 'POST',
    body: JSON.stringify({ song, cookie }),
  })
}

export function downloadMusicAsset(song: MusicSong, action: 'audio' | 'cover' | 'lyrics', cookie = '') {
  return downloadRequest('/api/music/download', {
    method: 'POST',
    body: JSON.stringify({ action, song, cookie }),
  })
}

export function getVideoInfo(url: string, platform?: string) {
  return request<VideoInfo>('/api/video/info/verified', {
    method: 'POST',
    headers: behaviorCaptchaHeaders(),
    body: JSON.stringify({ url, platform }),
  })
}

export function downloadVideo(payload: { url: string; platform?: string; format?: string; name?: string; cover?: boolean }) {
  return downloadRequest('/api/video/download', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}
