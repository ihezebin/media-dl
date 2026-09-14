import {
  clampSidebarWidth,
  SIDEBAR_WIDTH_DEFAULT,
} from '../../config'

export const SIDEBAR_WIDTH_KEY = 'sidebar-width'

export function loadSidebarWidth() {
  try {
    const raw = localStorage.getItem(SIDEBAR_WIDTH_KEY)
    if (!raw) return SIDEBAR_WIDTH_DEFAULT
    const n = Number(raw)
    if (Number.isFinite(n)) return clampSidebarWidth(n)
  } catch {
    /* ignore */
  }
  return SIDEBAR_WIDTH_DEFAULT
}

export function saveSidebarWidth(width: number) {
  try {
    localStorage.setItem(SIDEBAR_WIDTH_KEY, String(width))
  } catch {
    /* ignore */
  }
}
