import classNames from 'classnames'
import type { ReactNode } from 'react'
import { InboxOutlined } from '@ant-design/icons'

import styles from './index.module.scss'

type NiceEmptyProps = {
  title?: string
  description?: string
  /** 可选操作区（按钮等） */
  action?: ReactNode
  /** 铺满父容器，适合整页 / 列表空态 */
  cover?: boolean
  className?: string
}

/**
 * 空数据渲染：列表无数据、搜索无结果、整页空态等场景使用。
 */
export function NiceEmpty({
  title = '暂无数据',
  description = '这里还什么都没有，稍后再来看看吧',
  action,
  cover = false,
  className,
}: NiceEmptyProps) {
  return (
    <div className={classNames(styles.root, cover && styles.cover, className)}>
      <div className={styles.stage}>
        <div className={styles.visual} aria-hidden>
          <span className={styles.halo} />
          <span className={styles.orbit} />
          <span className={styles.orbitInner} />
          <span className={classNames(styles.dot, styles.dotA)} />
          <span className={classNames(styles.dot, styles.dotB)} />
          <span className={classNames(styles.dot, styles.dotC)} />
          <div className={styles.iconPlate}>
            <InboxOutlined className={styles.icon} />
          </div>
        </div>
        <h3 className={styles.title}>{title}</h3>
        <p className={styles.desc}>{description}</p>
        {action ? <div className={styles.action}>{action}</div> : null}
      </div>
    </div>
  )
}

export default NiceEmpty
