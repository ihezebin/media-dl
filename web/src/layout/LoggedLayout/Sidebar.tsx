import {
  DownOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  MoonOutlined,
  RightOutlined,
  SunOutlined,
} from '@ant-design/icons'
import { Avatar, Button, Dropdown, Popover, Typography } from 'antd'
import type { MenuProps } from 'antd'
import classNames from 'classnames'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'

import { AmbientBg } from '../../components/AmbientBg'
import { appConfig } from '../../config'
import { useStore } from '../../store'

import { menuConfig, type LayoutMenuItem } from './menu.config'
import styles from './styles.module.scss'

const { Text } = Typography

function BrandBar({ collapsed }: { collapsed: boolean }) {
  const navigate = useNavigate()
  const setSidebarCollapsed = useStore((s) => s.setSidebarCollapsed)

  return (
    <div className={styles.sidebarBrand}>
      <button
        type="button"
        className={styles.sidebarBrandLeft}
        aria-label="返回首页"
        title="返回首页"
        onClick={() => navigate('/')}>
        <img className={styles.sidebarLogo} src="/logo.svg" alt="" />
        {!collapsed ? <span className={styles.sidebarBrandName}>{appConfig.title}</span> : null}
      </button>
      {!collapsed ? (
        <Button
          className={styles.sidebarCollapseBtn}
          type="default"
          shape="circle"
          size="small"
          icon={<MenuFoldOutlined />}
          aria-label="收起侧边栏"
          title="收起侧边栏"
          onClick={() => setSidebarCollapsed(true)}
        />
      ) : null}
    </div>
  )
}

function buildPath(parentKeys: string[], key: string) {
  return ('/' + [...parentKeys, key].join('/')).replace(/\/+/g, '/')
}

function isPathActive(pathname: string, path: string) {
  return pathname === path || pathname.startsWith(path + '/')
}

function MenuLeaf({
  item,
  path,
  active,
  onNavigate,
}: {
  item: LayoutMenuItem
  path: string
  active: boolean
  onNavigate: (path: string) => void
}) {
  return (
    <button
      type="button"
      className={classNames(styles.settingsNavItem, active && styles.active)}
      onClick={() => onNavigate(path)}
      title={item.label}
      aria-label={item.label}>
      {item.icon ? <span className={styles.settingsNavIcon}>{item.icon}</span> : null}
      <span className={styles.settingsNavLabel}>{item.label}</span>
    </button>
  )
}

function MenuGroupChildren({
  item,
  parentKeys,
  pathname,
  onNavigate,
  indented = true,
}: {
  item: LayoutMenuItem
  parentKeys: string[]
  pathname: string
  onNavigate: (path: string) => void
  indented?: boolean
}) {
  return (
    <div
      className={classNames(
        styles.sidebarMenuGroupChildren,
        !indented && styles.sidebarMenuGroupChildrenFlat,
      )}>
      {(item.children ?? []).map((child) => {
        const path = buildPath([...parentKeys, item.key], child.key)
        return (
          <button
            key={child.key}
            type="button"
            className={classNames(
              styles.projectItem,
              isPathActive(pathname, path) && styles.active,
            )}
            onClick={() => onNavigate(path)}>
            <span className={styles.projectItemName} title={child.label}>
              {child.label}
            </span>
          </button>
        )
      })}
    </div>
  )
}

function MenuGroup({
  item,
  parentKeys,
  pathname,
  collapsed,
  onNavigate,
}: {
  item: LayoutMenuItem
  parentKeys: string[]
  pathname: string
  collapsed: boolean
  onNavigate: (path: string) => void
}) {
  const childActive = (item.children ?? []).some((child) =>
    isPathActive(pathname, buildPath([...parentKeys, item.key], child.key)),
  )
  const [open, setOpen] = useState(true)
  const [popupOpen, setPopupOpen] = useState(false)

  useEffect(() => {
    if (childActive) setOpen(true)
  }, [childActive])

  useEffect(() => {
    if (!collapsed) setPopupOpen(false)
  }, [collapsed])

  if (collapsed) {
    return (
      <Popover
        trigger="hover"
        placement="rightTop"
        mouseEnterDelay={0.08}
        mouseLeaveDelay={0.12}
        arrow={{ pointAtCenter: false }}
        open={popupOpen}
        onOpenChange={setPopupOpen}
        rootClassName={styles.sidebarSubmenuPopover}
        destroyOnHidden
        content={
          <div className={styles.sidebarSubmenuPanel}>
            <div className={styles.sidebarSubmenuTitle}>{item.label}</div>
            <MenuGroupChildren
              item={item}
              parentKeys={parentKeys}
              pathname={pathname}
              indented={false}
              onNavigate={(path) => {
                setPopupOpen(false)
                onNavigate(path)
              }}
            />
          </div>
        }>
        <button
          type="button"
          className={classNames(
            styles.settingsNavItem,
            styles.sidebarMenuGroupTrigger,
            childActive && styles.isActiveGroup,
            popupOpen && styles.isPopupOpen,
          )}
          title={item.label}
          aria-label={item.label}
          aria-haspopup="dialog"
          aria-expanded={popupOpen}>
          {item.icon ? <span className={styles.settingsNavIcon}>{item.icon}</span> : null}
          <span className={styles.settingsNavLabel}>{item.label}</span>
        </button>
      </Popover>
    )
  }

  return (
    <div className={styles.sidebarMenuGroup}>
      <button
        type="button"
        className={classNames(
          styles.settingsNavItem,
          styles.sidebarMenuGroupTrigger,
          childActive && styles.isActiveGroup,
        )}
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}>
        {item.icon ? <span className={styles.settingsNavIcon}>{item.icon}</span> : null}
        <span className={styles.settingsNavLabel}>{item.label}</span>
        <span className={styles.sidebarMenuGroupArrow}>
          {open ? <DownOutlined /> : <RightOutlined />}
        </span>
      </button>
      {open ? (
        <MenuGroupChildren
          item={item}
          parentKeys={parentKeys}
          pathname={pathname}
          onNavigate={onNavigate}
        />
      ) : null}
    </div>
  )
}

function renderMenuItems(
  items: LayoutMenuItem[],
  parentKeys: string[],
  pathname: string,
  collapsed: boolean,
  onNavigate: (path: string) => void,
): ReactNode {
  return items.map((item) => {
    if (item.children?.length) {
      return (
        <MenuGroup
          key={item.key}
          item={item}
          parentKeys={parentKeys}
          pathname={pathname}
          collapsed={collapsed}
          onNavigate={onNavigate}
        />
      )
    }
    const path = buildPath(parentKeys, item.key)
    return (
      <MenuLeaf
        key={item.key}
        item={item}
        path={path}
        active={isPathActive(pathname, path)}
        onNavigate={onNavigate}
      />
    )
  })
}

function UserFooter({ collapsed }: { collapsed: boolean }) {
  const navigate = useNavigate()
  const user = useStore((s) => s.user)
  const logout = useStore((s) => s.logout)
  const themeDark = useStore((s) => s.themeDark)
  const setThemeDark = useStore((s) => s.setThemeDark)
  const setSidebarCollapsed = useStore((s) => s.setSidebarCollapsed)
  const name = user?.username || '访客'
  const initial = name.slice(0, 1).toUpperCase()

  const items: MenuProps['items'] = [
    {
      key: 'theme',
      icon: themeDark ? <SunOutlined /> : <MoonOutlined />,
      label: themeDark ? '切换明亮主题' : '切换暗黑主题',
      onClick: () => setThemeDark(!themeDark),
    },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      danger: true,
      onClick: () => {
        logout()
        navigate('/login', { replace: true })
      },
    },
  ]

  return (
    <div className={styles.sidebarFooterStack}>
      <Dropdown menu={{ items }} trigger={['click']} placement={collapsed ? 'topLeft' : 'topRight'}>
        <button
          type="button"
          className={styles.sidebarUserBtn}
          title="账号菜单"
          aria-label={`账号菜单：${name}`}>
          <Avatar className={styles.sidebarUserAvatar} size={36} src={user?.avatar}>
            {initial}
          </Avatar>
          <span className={styles.sidebarUserMeta}>
            <Text className={styles.sidebarUserName} ellipsis>
              {name}
            </Text>
            <Text className={styles.sidebarUserHint} type="secondary">
              已登录
            </Text>
          </span>
          <DownOutlined className={styles.sidebarUserCaret} />
        </button>
      </Dropdown>
      {collapsed ? (
        <Button
          className={styles.sidebarExpandBtn}
          type="default"
          shape="circle"
          size="small"
          icon={<MenuUnfoldOutlined />}
          aria-label="展开侧边栏"
          title="展开侧边栏"
          onClick={() => setSidebarCollapsed(false)}
        />
      ) : null}
    </div>
  )
}

export function Sidebar() {
  const navigate = useNavigate()
  const location = useLocation()
  const collapsed = useStore((s) => s.sidebarCollapsed)
  const items = useMemo(() => menuConfig, [])

  return (
    <aside className={classNames(styles.sidebar, collapsed && styles.sidebarIsCollapsed)}>
      <AmbientBg variant="sidebar" />
      <div className={classNames(styles.sidebarHeader, styles.sidebarHeaderStack)}>
        <BrandBar collapsed={collapsed} />
      </div>
      <div className={styles.sidebarBody}>
        {renderMenuItems(items, [], location.pathname, collapsed, navigate)}
      </div>
      <div className={styles.sidebarFooter}>
        <UserFooter collapsed={collapsed} />
      </div>
    </aside>
  )
}
