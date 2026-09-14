import { useNavigate, useLocation, useRoutes } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { KEY_TOKEN } from '@hezebin/doraemon'
import { Spin } from 'antd'

import { useStore } from '../store'

import { routeConfig } from './config'

const Router = () => {
  const setToken = useStore((s) => s.setToken)
  const [ready, setReady] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()

  useEffect(() => {
    const searchParams = new URLSearchParams(location.search)
    const newToken = searchParams.get(KEY_TOKEN)
    if (newToken) {
      setToken(newToken)
      searchParams.delete(KEY_TOKEN)
      const newSearch = searchParams.toString()
      navigate(location.pathname + (newSearch ? '?' + newSearch : '') + location.hash, {
        replace: true,
      })
      return
    }
    setReady(true)
  }, [location.pathname, location.search, location.hash, navigate, setToken])

  return (
    <Spin style={{ maxHeight: 'initial', height: '100vh' }} size="large" spinning={!ready}>
      {useRoutes(routeConfig)}
    </Spin>
  )
}

export default Router
