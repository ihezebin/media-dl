import { Outlet } from 'react-router-dom'

import style from './index.module.scss'

const GlobalLayout = () => {
  return (
    <div className={style.container}>
      <h1>GlobalLayout</h1>
      <Outlet />
    </div>
  )
}

export default GlobalLayout
