import { useRoutes } from 'react-router-dom'

import { routeConfig } from './config'
import styles from './index.module.scss'

const Router = () => {
  return <div className={styles.root}>{useRoutes(routeConfig)}</div>
}

export default Router
