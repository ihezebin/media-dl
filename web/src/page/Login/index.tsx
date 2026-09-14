import { LockOutlined, UserOutlined } from '@ant-design/icons'
import { Button, Form, Input, notification } from 'antd'
import classNames from 'classnames'
import { useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'

import { AmbientBg } from '../../components/AmbientBg'
import { appConfig } from '../../config'
import { useStore } from '../../store'

import styles from './index.module.scss'

type LoginForm = {
  username: string
  password: string
}

const Login = () => {
  const navigate = useNavigate()
  const location = useLocation()
  const login = useStore((s) => s.login)
  const [loading, setLoading] = useState(false)
  const brandMark = appConfig.title.toUpperCase()

  const from =
    (location.state as { from?: string } | null)?.from &&
    (location.state as { from?: string }).from !== '/login'
      ? (location.state as { from: string }).from
      : '/'

  const onFinish = (values: LoginForm) => {
    setLoading(true)
    window.setTimeout(() => {
      const username = values.username.trim()
      login(username)
      notification.success({
        message: '登录成功',
        description: `欢迎回来，${username}`,
      })
      setLoading(false)
      navigate(from, { replace: true })
    }, 420)
  }

  return (
    <div className={styles.loginPage}>
      <AmbientBg variant="page" />

      <div className={styles.decor} aria-hidden>
        <span className={classNames(styles.ring, styles.ringA)} />
        <span className={classNames(styles.ring, styles.ringB)} />
        <span className={classNames(styles.floatDot, styles.dot1)} />
        <span className={classNames(styles.floatDot, styles.dot2)} />
        <span className={classNames(styles.floatDot, styles.dot3)} />
        <span className={classNames(styles.floatDot, styles.dot4)} />
      </div>

      <div className={styles.loginPanel}>
        <div className={styles.panelGlow} aria-hidden />

        <div className={styles.loginBrand}>
          <div className={styles.loginLogoTrack}>
            <div className={styles.loginLogoWrap}>
              <img className={styles.loginLogo} src="/logo.svg" alt="" />
            </div>
          </div>
          <p className={styles.loginBrandMark} aria-label={brandMark}>
            {Array.from(brandMark).map((ch, i) => (
              <span
                key={`${i}-${ch}`}
                className={styles.brandChar}
                style={{ animationDelay: `${0.35 + i * 0.05}s` }}>
                {ch === ' ' ? '\u00A0' : ch}
              </span>
            ))}
          </p>
          <h1 className={styles.loginTitle}>欢迎回来</h1>
          <p className={styles.loginSubtitle}>输入任意账号密码即可登录（演示）</p>
        </div>

        <Form
          className={styles.loginForm}
          name="login"
          size="large"
          onFinish={onFinish}
          autoComplete="off"
          requiredMark={false}>
          <Form.Item
            className={styles.fieldEnter}
            style={{ animationDelay: '0.45s' }}
            name="username"
            rules={[
              { required: true, message: '请输入账号' },
              { whitespace: true, message: '请输入账号' },
            ]}>
            <Input
              prefix={<UserOutlined className={styles.loginInputIcon} />}
              placeholder="账号"
              allowClear
            />
          </Form.Item>
          <Form.Item
            className={styles.fieldEnter}
            style={{ animationDelay: '0.58s' }}
            name="password"
            rules={[
              { required: true, message: '请输入密码' },
              { whitespace: true, message: '请输入密码' },
            ]}>
            <Input.Password
              prefix={<LockOutlined className={styles.loginInputIcon} />}
              placeholder="密码"
            />
          </Form.Item>
          <Form.Item
            className={classNames(styles.loginSubmitItem, styles.fieldEnter)}
            style={{ animationDelay: '0.72s' }}>
            <Button
              type="primary"
              htmlType="submit"
              className={classNames(styles.primaryBtn, styles.loginSubmit)}
              block
              loading={loading}>
              <span className={styles.submitShine} aria-hidden />
              登录
            </Button>
          </Form.Item>
        </Form>

        <p className={styles.loginHint}>演示模式：任意非空内容均可登录成功</p>
      </div>
    </div>
  )
}

export default Login
