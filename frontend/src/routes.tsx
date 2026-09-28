import { Navigate, type RouteObject } from 'react-router'

import { LoginPage } from './features/auth/LoginPage'
import { RequireAuth } from './features/auth/RequireAuth'
import { DashboardPage } from './features/dashboard/DashboardPage'
import { PlaceholderPage } from './features/PlaceholderPage'
import { AppShell } from './layout/AppShell'
import type { RouteHandle } from './layout/crumbs'

export const routes: RouteObject[] = [
  { path: '/login', element: <LoginPage /> },
  {
    element: <RequireAuth />,
    children: [
      {
        element: <AppShell />,
        children: [
          { index: true, element: <DashboardPage />, handle: { crumb: 'Dashboard' } satisfies RouteHandle },
          { path: 'meters/*', element: <PlaceholderPage title="Medidores" />, handle: { crumb: 'Medidores' } satisfies RouteHandle },
          { path: 'anomalies/*', element: <PlaceholderPage title="Anomalías IA" />, handle: { crumb: 'Anomalías IA' } satisfies RouteHandle },
        ],
      },
    ],
  },
  { path: '*', element: <Navigate to="/" replace /> },
]
