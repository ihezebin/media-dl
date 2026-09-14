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
          colorPrimary: themeDark ? '#5b9dff' : '#1677ff',
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
