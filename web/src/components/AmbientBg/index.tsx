import classNames from 'classnames'

import styles from './index.module.scss'

type AmbientBgProps = {
  variant?: 'main' | 'sidebar' | 'page'
  className?: string
}

/** 首页动画背景：光球 + 网格 */
export function AmbientBg({ variant = 'main', className }: AmbientBgProps) {
  return (
    <div
      className={classNames(styles.root, styles[variant], className)}
      aria-hidden>
      <div className={classNames(styles.orb, styles.orbA)} />
      <div className={classNames(styles.orb, styles.orbB)} />
      <div className={styles.mesh} />
    </div>
  )
}
