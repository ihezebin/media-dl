import classNames from 'classnames'

import { AmbientBg } from '../AmbientBg'
import { BuildStage } from '../BuildStage'

import styles from './index.module.scss'

type PageLoadingProps = {
  /** Suspense 全屏；后台菜单预览时留空，铺满主内容区 */
  fullscreen?: boolean
}

/** 懒加载动画：首页氛围背景 + 构建圆圈动画 */
export function PageLoading({ fullscreen = false }: PageLoadingProps) {
  return (
    <div className={classNames(styles.root, fullscreen && styles.fullscreen)}>
      <AmbientBg variant={fullscreen ? 'page' : 'main'} />
      <div className={styles.content}>
        <BuildStage title="加载中..." description="请稍候，资源准备中…" />
      </div>
    </div>
  )
}

export default PageLoading
