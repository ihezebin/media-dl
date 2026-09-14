import type { RouteObject } from 'react-router-dom'
import { Navigate } from 'react-router-dom'

import ContentLoading from '../components/ContentLoading'
import NiceEmpty from '../components/NiceEmpty'
import PageLoading from '../components/PageLoading'
import LoggedLayout from '../layout/LoggedLayout'
import BuildAnimation from '../page/BuildAnimation'
import EmptyHome from '../page/EmptyHome'
import Forbidden from '../page/Forbidden'
import Login from '../page/Login'
import Nothing from '../page/Nothing'
import Test from '../page/Test'

import { GuestOnly, RequireAuth } from './AuthGate'

export const routeConfig: RouteObject[] = [
  {
    path: 'login',
    element: (
      <GuestOnly>
        <Login />
      </GuestOnly>
    ),
  },
  {
    element: (
      <RequireAuth>
        <LoggedLayout />
      </RequireAuth>
    ),
    children: [
      { index: true, element: <Navigate to={'example/home_animation'} replace /> },
      { path: 'example/home_animation', element: <EmptyHome /> },
      { path: 'example/build_animation', element: <BuildAnimation /> },
      { path: 'example/page_loading', element: <PageLoading /> },
      { path: 'example/content_loading', element: <ContentLoading /> },
      { path: 'example/empty', element: <NiceEmpty cover /> },
      { path: 'need_auth', element: <Test /> },
      { path: 'test/:id', element: <Test /> },
      { path: 'nothing', element: <Nothing /> },
      { path: 'forbidden', element: <Forbidden /> },
      { path: '*', element: <Navigate to={'nothing'} replace /> },
    ],
  },
]
