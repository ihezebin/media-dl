export type AppFontKey = 'zhuque_fangsong' | 'lxgw_wenkai' | 'xiaolai'

export const DEFAULT_APP_FONT: AppFontKey = 'xiaolai'

export const FONT_FAMILIES: Record<AppFontKey, string> = {
  zhuque_fangsong: '"Zhuque Fangsong", "STFangsong", "FangSong", "宋体", "Songti SC", "PingFang SC", serif',
  lxgw_wenkai: '"LXGW WenKai", "Kaiti SC", "KaiTi", "楷体", "Songti SC", "PingFang SC", serif',
  xiaolai: '"Xiaolai SC", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif',
}

const FONT_LOADERS: Record<AppFontKey, () => Promise<unknown>> = {
  zhuque_fangsong: () => import('@free-fonts/zhuque-fangsong/zhuque-fangsong.css'),
  lxgw_wenkai: () => Promise.all([
    import('@hanzi.pro/webfonts-lxgw-wenkai/swap/400.css'),
    import('@hanzi.pro/webfonts-lxgw-wenkai/swap/500.css'),
  ]),
  xiaolai: () => import('@chinese-fonts/xiaolai/dist/Xiaolai/result.css'),
}

const loadedFonts = new Set<AppFontKey>()

export function loadAppFont(font: AppFontKey) {
  if (loadedFonts.has(font)) return Promise.resolve()
  return FONT_LOADERS[font]().then(() => {
    loadedFonts.add(font)
  })
}

export function isAppFontKey(value: string | null | undefined): value is AppFontKey {
  return value != null && value in FONT_FAMILIES
}
