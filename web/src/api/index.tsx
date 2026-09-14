import { getLocalItem, KEY_TOKEN, newApi, setLocalItem } from '@hezebin/doraemon'
import { notification } from 'antd'

const baseURL = '/api'
const timeout = import.meta.env.PROD ? 10000 : 0

export const api = newApi({
  baseURL: baseURL,
  timeout: timeout,
  withToken: () => {
    return getLocalItem(KEY_TOKEN)
  },
  onResponse: (res) => {
    if (res?.code !== 0 && res?.code !== 1) {
      notification.error({
        message: '请求失败',
        description: res?.message || '未知错误',
      })
    }
    return res
  },
  onError: (res) => {
    console.log('onError')
    if (res.status === 401) {
      setLocalItem(KEY_TOKEN)
    } else if (res?.message) {
      notification.error({
        message: '请求失败',
        description: res.message,
      })
    } else {
      const resp = res.response
      notification.error({
        message: resp?.statusText || '请求失败',
        description: `错误码: ${resp?.status ?? '—'}`,
      })
    }
    return res
  },
  onAbnormal: (err, code, msg) => {
    console.error('onAbnormal')
    notification.error({
      message: msg || '网络异常',
      description: `错误码: ${code}`,
    })
    return err
  },
})
