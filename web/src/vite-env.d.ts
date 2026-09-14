/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_APP_TITLE: string
  readonly VITE_SIDEBAR_WIDTH_MIN: string
  readonly VITE_SIDEBAR_WIDTH_MAX: string
  readonly VITE_SIDEBAR_WIDTH_DEFAULT: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
