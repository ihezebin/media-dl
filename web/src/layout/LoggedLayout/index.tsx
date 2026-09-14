import classNames from 'classnames'
import { useState, type CSSProperties } from 'react'
import { Outlet } from 'react-router-dom'

import { AmbientBg } from '../../components/AmbientBg'
import { useStore } from '../../store'

import { Sidebar } from './Sidebar'
import { SidebarResizeHandle } from './SidebarResizeHandle'
import { loadSidebarWidth } from './sidebarWidth'
import styles from './styles.module.scss'

const LoggedLayout = () => {
  const sidebarCollapsed = useStore((s) => s.sidebarCollapsed)
  const [sidebarWidth, setSidebarWidth] = useState(() => loadSidebarWidth())

  return (
    <div
      className={classNames(styles.appShell, sidebarCollapsed && styles.sidebarCollapsed)}
      style={{ '--sidebar-width': `${sidebarWidth}px` } as CSSProperties}>
      <Sidebar />
      {!sidebarCollapsed && (
        <SidebarResizeHandle width={sidebarWidth} onWidthChange={setSidebarWidth} />
      )}
      <main className={styles.main}>
        <AmbientBg variant="main" />
        <div className={styles.mainContent}>
          <Outlet />
        </div>
      </main>
    </div>
  )
}

export default LoggedLayout
