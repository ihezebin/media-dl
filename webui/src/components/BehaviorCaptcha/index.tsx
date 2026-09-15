import GoCaptcha from 'go-captcha-react'
import 'go-captcha-react/dist/go-captcha-react.cjs.development.css'
import { Modal, Spin, message } from 'antd'
import { useCallback, useEffect, useState } from 'react'

import { getBehaviorCaptcha, verifyBehaviorCaptcha, type BehaviorCaptcha as BehaviorCaptchaData } from '../../api/media'

import styles from './index.module.scss'

type Point = { x: number; y: number }
function BehaviorCaptcha({ open, onClose, onVerified, onLoadingChange }: { open: boolean; onClose: () => void; onVerified: (token: string, expiresAt: string) => void; onLoadingChange: (loading: boolean) => void }) {
  const [data, setData] = useState<BehaviorCaptchaData | null>(null)
  const [loading, setLoading] = useState(false)
  const [verifying, setVerifying] = useState(false)

  const load = useCallback(() => {
    setLoading(true)
    onLoadingChange(true)
    void getBehaviorCaptcha().then(setData).catch((error: unknown) => {
      message.error(error instanceof Error ? error.message : '验证码加载失败')
      onClose()
    }).finally(() => {
      setLoading(false)
      onLoadingChange(false)
    })
  }, [onClose, onLoadingChange])

  useEffect(() => {
    if (open) load()
  }, [load, open])

  const submit = useCallback((payload: { x?: number; y?: number; angle?: number }, reset: () => void) => {
    if (!data || verifying) return
    setVerifying(true)
    void verifyBehaviorCaptcha({ id: data.id, type: data.type, ...payload }).then((result) => {
      if (!result.verified) throw new Error('验证码校验失败')
      onVerified(result.token, result.expires_at)
      message.success('验证成功')
    }).catch((error: unknown) => {
      reset()
      message.error(error instanceof Error ? error.message : '验证码校验失败，请重试')
    }).finally(() => setVerifying(false))
  }, [data, onVerified, verifying])

  const events = {
    close: onClose,
    refresh: load,
  }

  if (!open) return null
  const captcha = data ? data.type === 'slide' ? <GoCaptcha.Slide
        config={{ width: 300, height: 220 }}
        data={{ image: data.image, thumb: data.thumb, thumbX: data.thumb_x || 0, thumbY: data.thumb_y || 0, thumbWidth: data.thumb_width || 0, thumbHeight: data.thumb_height || 0 }}
        events={{ ...events, confirm: (point: Point, reset: () => void) => submit({ x: Math.round(point.x), y: Math.round(point.y) }, reset) }}
      /> : data.type === 'drag' ? <GoCaptcha.SlideRegion
        config={{ width: 300, height: 220 }}
        data={{ image: data.image, thumb: data.thumb, thumbX: data.thumb_x || 0, thumbY: data.thumb_y || 0, thumbWidth: data.thumb_width || 0, thumbHeight: data.thumb_height || 0 }}
        events={{ ...events, confirm: (point: Point, reset: () => void) => submit({ x: Math.round(point.x), y: Math.round(point.y) }, reset) }}
      /> : <GoCaptcha.Rotate
        config={{ width: 300, height: 220 }}
        data={{ image: data.image, thumb: data.thumb, thumbSize: data.thumb_size || 0, angle: data.angle || 0 }}
        events={{ ...events, confirm: (angle: number, reset: () => void) => submit({ angle: Math.round(angle) }, reset) }}
      /> : null

  return <Modal className={styles.modal} title="完成验证码" open={open} onCancel={onClose} footer={null} destroyOnClose centered maskClosable={false} width={390}>
    <div className={styles.stage} aria-busy={loading || verifying}>
      <div className={`${styles.captchaContent} ${data?.type === 'drag' ? styles.dragContent : ''}`}>{captcha}</div>
      {loading && <div className={styles.loadingOverlay}><div className={styles.loading}><Spin /><span>正在准备验证码…</span></div></div>}
    </div>
  </Modal>
}

export default BehaviorCaptcha
