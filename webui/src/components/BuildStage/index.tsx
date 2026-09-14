import { LoadingOutlined } from '@ant-design/icons'
import classNames from 'classnames'

import styles from './index.module.scss'

type BuildStageProps = {
  title?: string
  description?: string
  showBars?: boolean
  className?: string
}

/** 构建页圆圈轨道加载动效，可复用于 Suspense / 初始化 */
export function BuildStage({
  title = '正在初始化项目',
  description = '同步依赖、准备环境并执行构建，请稍候',
  showBars = true,
  className,
}: BuildStageProps) {
  return (
    <div className={classNames(styles.stage, className)} aria-live="polite">
      <div className={styles.stageBg} aria-hidden>
        <span className={classNames(styles.orb, styles.orbA)} />
        <span className={classNames(styles.orb, styles.orbB)} />
        <span className={classNames(styles.ring, styles.ringA)} />
        <span className={classNames(styles.ring, styles.ringB)} />
        <span className={classNames(styles.ring, styles.ringC)} />
        <span className={classNames(styles.dot, styles.dot1)} />
        <span className={classNames(styles.dot, styles.dot2)} />
        <span className={classNames(styles.dot, styles.dot3)} />
        <span className={classNames(styles.dot, styles.dot4)} />
      </div>
      <div className={styles.core}>
        <div className={styles.pulse}>
          <LoadingOutlined className={styles.icon} spin />
        </div>
        {title ? <p className={styles.title}>{title}</p> : null}
        {description ? <p className={styles.desc}>{description}</p> : null}
        {showBars ? (
          <div className={styles.bars} aria-hidden>
            <span />
            <span />
            <span />
            <span />
            <span />
          </div>
        ) : null}
      </div>
    </div>
  )
}
