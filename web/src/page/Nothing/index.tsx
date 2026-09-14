import { HomeOutlined } from '@ant-design/icons'
import { Button } from 'antd'
import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'

import { AmbientBg } from '../../components/AmbientBg'

import styles from './index.module.scss'

const Nothing = () => {
  const navigate = useNavigate()

  return (
    <div className={styles.statusPage}>
      <AmbientBg variant="page" />
      <div className={styles.statusPanel}>
        <div className={styles.statusCode} aria-hidden>
          404
        </div>
        <h1 className={styles.statusTitle}>页面不存在</h1>
        <p className={styles.statusDesc}>抱歉，您访问的地址没有对应页面，可能已被移动或输入有误。</p>
        <div className={styles.statusActions}>
          <Button
            type="primary"
            className={classNames(styles.primaryBtn, styles.statusCta)}
            icon={<HomeOutlined />}
            size="large"
            onClick={() => navigate('/')}>
            回到首页
          </Button>
          <Button className={styles.statusSecondary} size="large" onClick={() => navigate(-1)}>
            返回上一页
          </Button>
        </div>
      </div>
    </div>
  )
}

export default Nothing
