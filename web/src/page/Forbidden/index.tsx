import { HomeOutlined, LockOutlined } from '@ant-design/icons'
import { Button } from 'antd'
import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'

import { AmbientBg } from '../../components/AmbientBg'
import styles from '../Nothing/index.module.scss'

const Forbidden = () => {
  const navigate = useNavigate()

  return (
    <div className={styles.statusPage}>
      <AmbientBg variant="page" />
      <div className={styles.statusPanel}>
        <div className={classNames(styles.statusCode, styles.statusCodeWarn)} aria-hidden>
          403
        </div>
        <div className={styles.statusIconBadge} aria-hidden>
          <LockOutlined />
        </div>
        <h1 className={styles.statusTitle}>没有访问权限</h1>
        <p className={styles.statusDesc}>抱歉，当前账号无权查看此页面。如需开通权限，请联系管理员。</p>
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

export default Forbidden
