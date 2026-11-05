import type { RouteObject } from 'react-router-dom'

import Status from '../page/Status'
import LoggedLayout from '../layout/LoggedLayout'
import Home from '../page/Home'
import MusicSearch from '../page/MusicSearch'
import VideoDownload from '../page/VideoDownload'

export const routeConfig: RouteObject[] = [
  { path: '*', element: <Status code='404' /> },
  { path: '/401', element: <Status code='401' /> },
  { path: '/403', element: <Status code='403' /> },
  { path: '/404', element: <Status code='404' /> },
  { path: '/500', element: <Status code='500' /> },
  { path: '/error', element: <Status code='500' /> },

  {
    element: <LoggedLayout />,
    children: [
      { index: true, element: <Home /> },
      { path: 'video', element: <VideoDownload /> },
      { path: 'music', element: <MusicSearch /> },
    ],
  },
]
