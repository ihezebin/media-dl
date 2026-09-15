import { GithubOutlined, HomeOutlined, MoonOutlined, SunOutlined } from '@ant-design/icons'
import { Dropdown, Tooltip } from 'antd'
import type { MenuProps } from 'antd'
import { useLayoutEffect, useRef } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'

import { useStore } from '../../store'

import styles from './styles.module.scss'

const fontItems: MenuProps['items'] = [
  { key: 'xiaolai', label: '小赖字体' },
  { key: 'lxgw_wenkai', label: '霞鹜文楷' },
  { key: 'zhuque_fangsong', label: '朱雀仿宋' },
]

export default function LoggedLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const contentRef = useRef<HTMLDivElement>(null)
  const themeDark = useStore((s) => s.themeDark)
  const setThemeDark = useStore((s) => s.setThemeDark)
  const fontFamily = useStore((s) => s.fontFamily)
  const setFontFamily = useStore((s) => s.setFontFamily)

  useLayoutEffect(() => {
    const content = contentRef.current
    if (content) content.scrollTop = 0
  }, [location.pathname])

  return (
    <div className={styles.shell}>
      <main className={styles.main}>
        <header className={styles.topbar}>
          <button type="button" className={styles.homeButton} aria-label="返回首页" title="返回首页" onClick={() => navigate('/')}>
            <img src="/logo.svg" alt="" />
          </button>
          <div className={styles.topActions}>
            {location.pathname !== '/' && <Tooltip title="返回首页"><button type="button" className={styles.iconButton} aria-label="返回首页" onClick={() => navigate('/')}><HomeOutlined /></button></Tooltip>}
            <Dropdown menu={{ items: fontItems, selectedKeys: [fontFamily], onClick: ({ key }) => setFontFamily(key as typeof fontFamily) }} trigger={['click']} placement="bottomRight">
              <button type="button" className={styles.iconButton} aria-label="切换字体" title="切换字体"><span className={styles.fontIcon}>字</span></button>
            </Dropdown>
            <Tooltip title={themeDark ? '切换浅色主题' : '切换深色主题'}>
              <button type="button" className={styles.iconButton} aria-label="切换主题" onClick={() => setThemeDark(!themeDark)}>{themeDark ? <SunOutlined /> : <MoonOutlined />}</button>
            </Tooltip>
            <Tooltip title="打开 GitHub 项目">
              <button type="button" className={styles.iconButton} aria-label="打开 GitHub 项目" onClick={() => window.open('https://github.com/ihezebin/media-dl', '_blank', 'noopener,noreferrer')}><GithubOutlined /></button>
            </Tooltip>
          </div>
        </header>
        <div ref={contentRef} className={styles.content}>
          <Outlet />
          <footer className={styles.supportFooter}>
            <span>NCM 提供技术支持</span>
            <a href="https://ncm.hezebin.com">ncm.hezebin.com</a>
          </footer>
        </div>
      </main>
    </div>
  )
}
