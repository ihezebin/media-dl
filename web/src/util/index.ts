import { getLocalItem, KEY_TOKEN, setLocalItem } from '@hezebin/doraemon'

const TITLE_KEY = 'title'
export const setDocumentTitle = (subtitle?: string) => {
  const title = subtitle ? `${subtitle} - 河泽冰` : '河泽冰'
  document.title = title
  setLocalItem(TITLE_KEY, title)
}

export const getDocumentTitle = (): string => {
  return getLocalItem(TITLE_KEY) || '河泽冰'
}

export const handleUnAuthorized = (fn?: (() => void) | (() => void)[]) => {
  if (fn) {
    if (Array.isArray(fn)) {
      fn.forEach((f) => f())
    } else {
      fn()
    }
  }
  setLocalItem(KEY_TOKEN)
  document.location.href = '/login'
}
