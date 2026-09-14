function parsePx(raw: string | undefined, fallback: number) {
  const n = Number(raw)
  return Number.isFinite(n) && n > 0 ? Math.round(n) : fallback
}

/** 应用级运行时配置（来自 .env / Vite 环境变量） */
export const appConfig = {
  /** 系统名称 / 页面标题 */
  title: import.meta.env.VITE_APP_TITLE?.trim() || 'React Template Vite',

  sidebar: {
    widthMin: parsePx(import.meta.env.VITE_SIDEBAR_WIDTH_MIN, 250),
    widthMax: parsePx(import.meta.env.VITE_SIDEBAR_WIDTH_MAX, 520),
    widthDefault: parsePx(import.meta.env.VITE_SIDEBAR_WIDTH_DEFAULT, 288),
  },
} as const

/** 保证 default 落在 min~max 之间 */
export function clampSidebarWidth(width: number) {
  const { widthMin, widthMax } = appConfig.sidebar
  const min = Math.min(widthMin, widthMax)
  const max = Math.max(widthMin, widthMax)
  return Math.min(max, Math.max(min, Math.round(width)))
}

export const SIDEBAR_WIDTH_DEFAULT = clampSidebarWidth(appConfig.sidebar.widthDefault)
export const SIDEBAR_WIDTH_MIN = Math.min(appConfig.sidebar.widthMin, appConfig.sidebar.widthMax)
export const SIDEBAR_WIDTH_MAX = Math.max(appConfig.sidebar.widthMin, appConfig.sidebar.widthMax)
