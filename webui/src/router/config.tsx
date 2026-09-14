import type { RouteObject } from 'react-router-dom'

import LoggedLayout from '../layout/LoggedLayout'
import Home from '../page/Home'
import MusicSearch from '../page/MusicSearch'
import VideoDownload from '../page/VideoDownload'

export const routeConfig: RouteObject[] = [
  {
    element: <LoggedLayout />,
    children: [
      { index: true, element: <Home /> },
      { path: 'video', element: <VideoDownload /> },
      { path: 'music', element: <MusicSearch /> },
    ],
  },
]
