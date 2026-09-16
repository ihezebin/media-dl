import { useCallback, useRef, useState } from 'react'

import { forgetBehaviorCaptcha, hasBehaviorCaptchaToken, setBehaviorCaptchaToken } from '../../api/media'

import BehaviorCaptcha from './index'

type Action = () => Promise<void> | void

export function useBehaviorCaptcha() {
  const [open, setOpen] = useState(false)
  const [captchaLoading, setCaptchaLoading] = useState(false)
  const pendingAction = useRef<Action | null>(null)

  const runWithCaptcha = useCallback((action: Action) => {
    if (hasBehaviorCaptchaToken()) {
      void action()
      return
    }
    pendingAction.current = action
    setCaptchaLoading(true)
    setOpen(true)
  }, [])

  const onVerified = useCallback((token: string, expiresAt: string) => {
    setBehaviorCaptchaToken(token, expiresAt)
    setCaptchaLoading(false)
    setOpen(false)
    const action = pendingAction.current
    pendingAction.current = null
    if (action) void action()
  }, [])

  const close = useCallback(() => {
    forgetBehaviorCaptcha()
    setCaptchaLoading(false)
    pendingAction.current = null
    setOpen(false)
  }, [])

  const captcha = <BehaviorCaptcha open={open} onClose={close} onVerified={onVerified} onLoadingChange={setCaptchaLoading} />
  return { captcha, captchaLoading, captchaOpen: open, runWithCaptcha }
}
