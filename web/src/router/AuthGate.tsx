import { Navigate, useLocation } from 'react-router-dom'
import type { ReactNode } from 'react'

import { useStore } from '../store'

export function RequireAuth({ children }: { children: ReactNode }) {
  const token = useStore((s) => s.token)
  const location = useLocation()

  if (!token) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }

  return children
}

export function GuestOnly({ children }: { children: ReactNode }) {
  const token = useStore((s) => s.token)

  if (token) {
    return <Navigate to="/" replace />
  }

  return children
}
