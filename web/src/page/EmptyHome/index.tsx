import { ThunderboltOutlined } from '@ant-design/icons'
import { Button } from 'antd'
import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'

import { appConfig } from '../../config'

import styles from './index.module.scss'

/** 首页动画示例：主区已有氛围背景，本页只展示前景内容 */
const EmptyHome = () => {
  const navigate = useNavigate()
  const title = appConfig.title
  const brandMark = title.toUpperCase()

  return (
    <div className={styles.emptyHome}>
      <div className={styles.emptyHomePanel}>
        <div className={styles.emptyHomeBrandBlock}>
          <div className={styles.emptyHomeLogoTrack} aria-hidden>
            <img className={styles.emptyHomeLogo} src="/logo.svg" alt="" />
          </div>
          <p className={styles.emptyHomeBrand}>{brandMark}</p>
        </div>

        <h1 className={styles.emptyHomeTitle} aria-label={title}>
          {Array.from(title).map((ch, i) => (
            <span
              key={`${i}-${ch}`}
              className={styles.emptyHomeTitleChar}
              style={{ animationDelay: `${i * 0.08}s` }}>
              {ch === ' ' ? '\u00A0' : ch}
            </span>
          ))}
        </h1>
        <p className={styles.emptyHomeDesc}>
          首页欢迎动画示例：光球、网格、Logo 滚动与品牌字效。氛围背景已作为内容区默认背景复用。
        </p>

        <div className={styles.emptyHomeActions}>
          <Button
            type="primary"
            className={classNames(styles.primaryBtn, styles.emptyHomeCta)}
            icon={<ThunderboltOutlined />}
            size="large"
            onClick={() => navigate('/example/build_animation')}>
            查看构建动画
          </Button>
          <p className={styles.emptyHomeHint}>侧边栏「示例」可切换首页动画与构建动画</p>
        </div>

        <ul className={styles.emptyHomeSteps}>
          <li className={styles.emptyHomeStep}>
            <span className={styles.emptyHomeStepIndex}>01</span>
            <span className={styles.emptyHomeStepLabel}>品牌入场与光效</span>
          </li>
          <li className={styles.emptyHomeStepSep} aria-hidden />
          <li className={styles.emptyHomeStep}>
            <span className={styles.emptyHomeStepIndex}>02</span>
            <span className={styles.emptyHomeStepLabel}>标题逐字弹跳</span>
          </li>
          <li className={styles.emptyHomeStepSep} aria-hidden />
          <li className={styles.emptyHomeStep}>
            <span className={styles.emptyHomeStepIndex}>03</span>
            <span className={styles.emptyHomeStepLabel}>复用到业务空态</span>
          </li>
        </ul>
      </div>
    </div>
  )
}

export default EmptyHome
