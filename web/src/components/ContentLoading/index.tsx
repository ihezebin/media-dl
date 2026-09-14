import classNames from 'classnames'

import styles from './index.module.scss'

type ContentLoadingProps = {
  /** 铺满父容器（表格区、卡片列表、子页面等），默认 true */
  cover?: boolean
  className?: string
}

/**
 * 内容区加载动画：表格、卡片列表、子页面数据加载等场景使用。
 * 路由级懒加载请用 `PageLoading`。
 */
export function ContentLoading({ cover = true, className }: ContentLoadingProps) {
  return (
    <div
      className={classNames(styles.root, cover && styles.cover, className)}
      role="status"
      aria-live="polite"
      aria-busy="true"
      aria-label="加载中">
      <div className={styles.rings} aria-hidden>
        <span className={classNames(styles.ring, styles.ringA)} />
        <span className={classNames(styles.ring, styles.ringB)} />
        <span className={classNames(styles.ring, styles.ringC)} />
      </div>
    </div>
  )
}

export default ContentLoading
