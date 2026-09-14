import { ApiOutlined, EnvironmentOutlined, UserOutlined } from '@ant-design/icons'
import { Button, Tag } from 'antd'
import { useLocation, useParams } from 'react-router-dom'

import { api } from '../../api'
import { appConfig } from '../../config'
import { useStore } from '../../store'

import styles from './index.module.scss'

const Test = () => {
  const { user, setUser, clearUser } = useStore()
  const { id } = useParams()

  const location = useLocation()
  const searchParams = new URLSearchParams(location.search)
  const paramsObject: { [key: string]: string } = {}
  for (const [key, value] of searchParams.entries()) {
    paramsObject[key] = value
  }

  return (
    <div className={styles.page}>
      <div className={styles.panel}>
        <header className={styles.header}>
          <div>
            <h1 className={styles.title}>Test 示例页</h1>
            <p className={styles.desc}>演示全局状态、路由参数与 API 调用，样式同时适配明亮 / 暗黑主题。</p>
          </div>
          <Tag color="processing" className={styles.badge}>
            Demo
          </Tag>
        </header>

        <div className={styles.body}>
          <section className={styles.card}>
            <div className={styles.cardHead}>
              <span className={styles.cardIcon}>
                <UserOutlined />
              </span>
              <h2 className={styles.cardTitle}>用户状态</h2>
            </div>
            <p className={styles.cardMeta}>
              当前登录用户
              <strong className={styles.emphasis}>{user?.username ?? '—'}</strong>
            </p>
            <div className={styles.actions}>
              <Button
                type="primary"
                className={styles.primaryBtn}
                onClick={() =>
                  setUser({
                    id: user?.id || 'xxx',
                    username: 'new_username',
                    password_strength: 5,
                    email: user?.email || 'xxx',
                  })
                }>
                改变用户名
              </Button>
              <Button className={styles.ghostBtn} onClick={() => clearUser()}>
                清空用户
              </Button>
            </div>
          </section>

          <section className={styles.card}>
            <div className={styles.cardHead}>
              <span className={styles.cardIcon}>
                <EnvironmentOutlined />
              </span>
              <h2 className={styles.cardTitle}>环境与路由</h2>
            </div>
            <dl className={styles.kvList}>
              <div className={styles.kvRow}>
                <dt>环境变量</dt>
                <dd>
                  <code>{appConfig.title}</code>
                </dd>
              </div>
              <div className={styles.kvRow}>
                <dt>路径参数 id</dt>
                <dd>
                  <code>{id || '—'}</code>
                </dd>
              </div>
              <div className={styles.kvRow}>
                <dt>Query 参数</dt>
                <dd>
                  <code>{JSON.stringify(paramsObject)}</code>
                </dd>
              </div>
            </dl>
          </section>

          <section className={styles.card}>
            <div className={styles.cardHead}>
              <span className={styles.cardIcon}>
                <ApiOutlined />
              </span>
              <h2 className={styles.cardTitle}>API 请求</h2>
            </div>
            <p className={styles.cardMeta}>点击发送将请求 `/hello_401`，可在控制台查看响应。</p>
            <div className={styles.actions}>
              <Button
                type="primary"
                className={styles.primaryBtn}
                onClick={() => {
                  api
                    .get('/hello_401')
                    .then((res) => {
                      console.log('resp', res)
                    })
                    .catch((err) => {
                      console.log('err', err)
                    })
                }}>
                发送请求
              </Button>
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}

export default Test
