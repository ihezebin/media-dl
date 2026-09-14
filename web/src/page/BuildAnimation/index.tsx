import { BuildStage } from '../../components/BuildStage'

import styles from './index.module.scss'

/**
 * 「构建动画」示例页：面板外壳 + BuildStage 圆圈轨道动效。
 */
const BuildAnimation = () => {
  return (
    <div className={styles.page}>
      <div className={styles.panel}>
        <header className={styles.header}>
          <h2 className={styles.title}>构建动画</h2>
          <p className={styles.desc}>
            示例加载动效，可用于初始化、构建或长时间等待场景。动画由{' '}
            <code>BuildStage</code> 组件提供。
          </p>
        </header>
        <div className={styles.body}>
          <BuildStage
            className={styles.stage}
            title="正在初始化项目"
            description="同步依赖、准备环境并执行构建，请稍候；实际业务中可在此处展示进度文案"
          />
        </div>
      </div>
    </div>
  )
}

export default BuildAnimation
