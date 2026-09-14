import { BrowserRouter } from 'react-router-dom'
import { lazy, Suspense, useEffect } from 'react'

import PageLoading from '../components/PageLoading'
import { unsubscribeStore } from '../store'

const LazyComponent = lazy(() => import('./Router'))

const LazyRouter = () => {
  useEffect(() => {
    return () => {
      unsubscribeStore()
    }
  }, [])

  return (
    <BrowserRouter>
      <Suspense fallback={<PageLoading fullscreen />}>
        <LazyComponent />
      </Suspense>
    </BrowserRouter>
  )
}
export default LazyRouter
