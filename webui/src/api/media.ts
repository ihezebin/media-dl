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

export type FileResponse = {
  action: string
  file_url: string
  file_path: string
  cover_url?: string
  cover_path?: string
}

type Envelope<T> = { code: number; message?: string; data: T }

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
  })
  const payload = (await response.json().catch(() => null)) as Envelope<T> | null
  if (!response.ok || !payload || payload.code !== 0) {
    throw new Error(payload?.message || `请求失败 (${response.status})`)
  }
  return payload.data
}

export function getMusicPlatforms() {
  return request<{ platforms: string[] }>('/api/music/platforms')
}

export function searchMusic(params: { keyword: string; type: 'song' | 'artist' | 'album'; platforms: string[]; limit?: number; cookies?: Record<string, string> }) {
  return request<MusicSearchResponse>('/api/music/search', {
    method: 'POST',
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
  return request<FileResponse>('/api/music/download', {
    method: 'POST',
    body: JSON.stringify({ action, song, cookie }),
  })
}

export function getVideoInfo(url: string, platform?: string) {
  return request<VideoInfo>('/api/video/info', {
    method: 'POST',
    body: JSON.stringify({ url, platform }),
  })
}

export function downloadVideo(payload: { url: string; platform?: string; format?: string; name?: string; cover?: boolean }) {
  return request<FileResponse>('/api/video/download', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}
