import { Navigate, type RouteObject } from 'react-router'

import { AnomaliesPage } from './features/anomalies/AnomaliesPage'
import { InvestigationPage } from './features/anomalies/InvestigationPage'
import { LoginPage } from './features/auth/LoginPage'
import { RequireAuth } from './features/auth/RequireAuth'
import { DashboardPage } from './features/dashboard/DashboardPage'
import { MeterDetailPage } from './features/meters/MeterDetailPage'
import { MetersPage } from './features/meters/MetersPage'
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
          {
            path: 'meters',
            handle: { crumb: 'Medidores' } satisfies RouteHandle,
            children: [
              { index: true, element: <MetersPage /> },
              { path: ':meterId', element: <MeterDetailPage />, handle: { crumb: (p) => p.meterId ?? '' } satisfies RouteHandle },
            ],
          },
          {
            path: 'anomalies',
            handle: { crumb: 'Anomalías IA' } satisfies RouteHandle,
            children: [
              { index: true, element: <AnomaliesPage /> },
              { path: ':id', element: <InvestigationPage />, handle: { crumb: (p) => `#${p.id ?? ''}` } satisfies RouteHandle },
            ],
          },
        ],
      },
    ],
  },
  { path: '*', element: <Navigate to="/" replace /> },
]
