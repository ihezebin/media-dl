import { HomeOutlined, UserOutlined, ReloadOutlined, MoonOutlined, SunOutlined } from '@ant-design/icons'
import type { ReactNode } from 'react'

import { useStore } from '../../store'
import './style.css'

export type StatusCode = '401' | '403' | '404' | '500'
type Props = { code?: StatusCode; description?: string; children?: ReactNode; onLogin?: () => void; loginBusy?: boolean; onRetry?: () => void }
export default function StatusPage({ code = '404', description, children, onLogin, loginBusy = false, onRetry }: Props) {
  const dark = useStore(s => s.themeDark)
  const toggle = () => useStore.getState().setThemeDark(!dark)
  const home = '/'
  const retry = () => {
    const next = new URLSearchParams(location.search).get('next')
    const safe = next && next.startsWith('/') && !next.startsWith('//') && !next.includes('\\') && !/^\/(m\/)?(401|403|404|500|error)([/?#]|$)/.test(next)
    location.replace(safe ? next : home)
  }
  const content = {
    '403': ['这片领地，仅向管理员开放。', '当前账号没有管理权限，你可以返回账号管理或官网首页。'],
    '401': ['这片领地，需要一把钥匙。', '你暂时没有访问权限，先回首页逛逛吧。请尝试去登录认证。'],
    '404': ['咦，走进了猫咪的秘密角落。', '这个页面不存在，或已经搬去了新的地方。'],
    '500': ['服务器打了个小盹。', '暂时遇到了一点问题，请稍后再试。'],
  }[code]
  const mobileContent = {
    '401': ['请先登录', '登录后，再继续你的探索。请尝试去登录认证。'],
    '403': ['暂无访问权限', '当前账号没有管理权限，可以返回账号中心。'],
    '404': ['走进了猫咪的秘密角落', '这个页面不存在，或已经搬去了新的地方。'],
    '500': ['服务器打了个小盹。', '暂时无法连接服务，请稍后重试。'],
  }[code]
  return (
    <div className="doorway-status-page">
      <header className="status-header">
        <a href={home} className="status-mobile-home" aria-label="返回首页"><HomeOutlined /></a>
        <strong className="status-mobile-title">河泽冰</strong>
        <a href={home} className="status-brand" aria-label="返回首页"><img src="/headphone-cat-logo.webp" alt="项目 Logo" /><span>HEZEBIN<span className="status-brand-dot">.</span></span></a>
        <div className="status-appearance"><button type="button" className="status-button" aria-label="切换主题" onClick={toggle}>{dark ? <MoonOutlined /> : <SunOutlined />}</button></div>
      </header>
      <main>
        {children}
        <div className="status-art"><strong>{code}</strong><img src="/headphone-cat-logo.webp" alt="项目 Logo" /></div>
        <span className="status-eyebrow">{code === '404' ? 'A LITTLE LOST, STILL CURIOUS' : code === '401' ? 'A PRIVATE LITTLE CORNER' : 'TAKING A LITTLE CATNAP'}</span>
        <h1><span className="status-copy-desktop">{content[0]}</span><span className="status-copy-mobile">{mobileContent[0]}</span></h1>
        <p>{description || <><span className="status-copy-desktop">{content[1]}</span><span className="status-copy-mobile">{mobileContent[1]}</span></>}</p>
        <div className="status-actions">
          {code === '401' && <button type="button" className="status-button primary" disabled={loginBusy} onClick={onLogin || (() => location.assign('/music'))}><UserOutlined /> 前往登录</button>}
          {code === '403' && <a href="/music" className="status-button primary"><UserOutlined /> 账号管理</a>}
          <a href={home} className={code === '401' ? 'status-button' : 'status-button primary'}><HomeOutlined /> 返回首页</a>
          {code === '500' && <button type="button" className="status-button" onClick={onRetry || retry}><ReloadOutlined /> 重新加载</button>}
        </div>
      </main>
    </div>
  )
}
