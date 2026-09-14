import {
  ApiOutlined,
  AppstoreAddOutlined,
  HomeOutlined,
  InboxOutlined,
  LoadingOutlined,
  SyncOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'
import type { ReactNode } from 'react'

export type LayoutMenuItem = {
  key: string
  label: string
  icon?: ReactNode
  children?: LayoutMenuItem[]
}

export const menuConfig: LayoutMenuItem[] = [
  {
    icon: <AppstoreAddOutlined />,
    key: 'example',
    label: '示例',
    children: [
      { icon: <HomeOutlined />, key: 'home_animation', label: '首页动画' },
      { icon: <ThunderboltOutlined />, key: 'build_animation', label: '构建动画' },
      { icon: <LoadingOutlined />, key: 'page_loading', label: '懒加载动画' },
      { icon: <SyncOutlined />, key: 'content_loading', label: '内容加载' },
      { icon: <InboxOutlined />, key: 'empty', label: '空数据' },
    ],
  },
  { icon: <ApiOutlined />, key: 'test/1', label: 'TEST' },
  { icon: <ApiOutlined />, key: 'nothing', label: '404页面' },
  { icon: <ApiOutlined />, key: 'forbidden', label: '403页面' },
]
