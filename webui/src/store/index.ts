import { create } from 'zustand'
import { mountStoreDevtool } from 'simple-zustand-devtools'
import { getLocalItem, setLocalItem } from '@hezebin/doraemon'

import { DEFAULT_APP_FONT, FONT_FAMILIES, isAppFontKey, loadAppFont, type AppFontKey } from '../fonts/appFont'

interface IStore {
  themeDark: boolean
  setThemeDark: (dark: boolean) => void
  fontFamily: AppFontKey
  setFontFamily: (font: IStore['fontFamily']) => void
}

const KEY_THEME = 'theme'
const KEY_FONT = 'font-family'
const THEME_LIGHT = 'light'
const THEME_DARK = 'dark'
const THEME_DEFAULT = THEME_LIGHT
function setFontAttribute(font: IStore['fontFamily']) {
  document.documentElement.setAttribute('data-font', font)
  document.documentElement.style.setProperty('--app-font-family', FONT_FAMILIES[font])
}

export const useStore = create<IStore>((set) => ({
  themeDark: (() => {
    const theme = getLocalItem(KEY_THEME) || THEME_DEFAULT
    const dark = theme === THEME_DARK
    document.documentElement.setAttribute(KEY_THEME, theme)
    return dark
  })(),
  fontFamily: (() => {
    const stored = getLocalItem(KEY_FONT)
    const selected = isAppFontKey(stored) ? stored : DEFAULT_APP_FONT
    setFontAttribute(selected)
    return selected
  })(),
  setFontFamily: (font) => {
    set((state) => ({ ...state, fontFamily: font }))
    setFontAttribute(font)
    void loadAppFont(font)
  },
  setThemeDark: (dark: boolean) => {
    set((state) => ({ ...state, themeDark: dark }))
    document.documentElement.setAttribute(KEY_THEME, dark ? THEME_DARK : THEME_LIGHT)
  },
}))

export const unsubscribeStore = useStore.subscribe((state: IStore) => {
  if (state.themeDark) {
    setLocalItem(KEY_THEME, THEME_DARK)
  } else {
    setLocalItem(KEY_THEME, THEME_LIGHT)
  }

  setLocalItem(KEY_FONT, state.fontFamily)
})

if (import.meta.env.DEV) {
  mountStoreDevtool('Store', useStore)
}
