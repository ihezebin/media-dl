/** 应用级运行时配置（来自 .env / Vite 环境变量） */
export const appConfig = {
  /** 系统名称 / 页面标题 */
  title: import.meta.env.VITE_APP_TITLE?.trim() || 'media-dl',
} as const
