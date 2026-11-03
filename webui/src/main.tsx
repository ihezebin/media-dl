import { createRoot } from 'react-dom/client'
import { ConfigProvider, theme } from 'antd'
import zhCN from 'antd/locale/zh_CN'

import { DEFAULT_APP_FONT, loadAppFont } from './fonts/appFont'
import './assets/css/global.scss'
import { appConfig } from './config'
import LazyRouter from './router'
import { useStore } from './store'

document.title = appConfig.title

function AppRoot() {
  const themeDark = useStore((s) => s.themeDark)

  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: themeDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: {
          fontFamily: 'var(--app-font-family)',
          fontFamilyCode: 'var(--app-font-family)',
          colorPrimary: themeDark ? '#b6a1ed' : '#7860c8',
          colorInfo: themeDark ? '#b6a1ed' : '#7860c8',
          colorTextBase: themeDark ? '#ffffff' : '#45414f',
          colorText: themeDark ? '#e7e4ef' : '#45414f',
          colorTextHeading: themeDark ? '#e7e4ef' : '#45414f',
          colorTextSecondary: themeDark ? '#a19bad' : '#878491',
          colorBorder: themeDark ? '#302b3c' : '#e9e5ef',
          colorBgContainer: themeDark ? '#1d1a25' : '#ffffff',
          colorBgElevated: themeDark ? '#1d1a25' : '#ffffff',
          colorBgLayout: themeDark ? '#14121b' : '#f8f7fa',
          borderRadius: 9,
        },
      }}>
      <LazyRouter />
    </ConfigProvider>
  )
}

async function bootstrap() {
  await loadAppFont(DEFAULT_APP_FONT).catch(() => undefined)
  createRoot(document.getElementById('root')!).render(<AppRoot />)
}

void bootstrap()
