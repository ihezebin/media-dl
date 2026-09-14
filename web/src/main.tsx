import { createRoot } from 'react-dom/client'
import { ConfigProvider, notification, theme } from 'antd'
import zhCN from 'antd/locale/zh_CN'

import './assets/css/global.scss'
import { appConfig } from './config'
import LazyRouter from './router'
import { useStore } from './store'

const APP_FONT_FAMILY = '"华文楷体", "STKaiti", "Kaiti SC", "KaiTi", "楷体", serif'

document.title = appConfig.title

const content = `
 _                      _     _
| |                    | |   (_)
| | _   ____ _____ ____| | _  _ ____
| || \\ / _  |___  ) _  ) || \\| |  _ \\
| | | ( (/ / / __( (/ /| |_) ) | | | |
|_| |_|\\____|_____)____)____/|_|_| |_|
        `
console.log(content)

notification.config({
  placement: 'topRight',
  duration: 3,
  maxCount: 3,
})

function AppRoot() {
  const themeDark = useStore((s) => s.themeDark)

  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: themeDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: {
          fontFamily: APP_FONT_FAMILY,
          fontFamilyCode: APP_FONT_FAMILY,
          colorPrimary: themeDark ? '#5b9dff' : '#1677ff',
        },
      }}>
      <LazyRouter />
    </ConfigProvider>
  )
}

createRoot(document.getElementById('root')!).render(<AppRoot />)
