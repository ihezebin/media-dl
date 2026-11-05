import StatusPage from '../../components/StatusPage'
import type { StatusCode } from '../../components/StatusPage'

export default function Status({ code = '404' }: { code?: StatusCode }) {
  return <StatusPage code={code} />
}
