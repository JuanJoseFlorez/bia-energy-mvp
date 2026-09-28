import { Navigate, Outlet, useLocation } from 'react-router'

import { useAuth } from './authContext'

/** Renders the child routes only with a session; otherwise sends the user to /login and back afterwards. */
export function RequireAuth() {
  const { session } = useAuth()
  const location = useLocation()
  if (!session) return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />
  return <Outlet />
}
