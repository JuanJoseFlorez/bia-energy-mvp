import { QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router'

import { createQueryClient } from '../api/queries'
import type { Session } from '../api/types'
import { AuthProvider, SESSION_KEY } from '../features/auth/AuthProvider'
import { routes } from '../routes'

export const demoSession: Session = { token: 'a'.repeat(64), user: { username: 'demo' } }

/** Renders the real route table at path, optionally signed in. */
export function renderApp(path: string, { session }: { session?: Session } = {}) {
  if (session) localStorage.setItem(SESSION_KEY, JSON.stringify(session))
  const queryClient = createQueryClient()
  queryClient.setDefaultOptions({ queries: { ...queryClient.getDefaultOptions().queries, retry: false } })
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <RouterProvider router={router} />
      </AuthProvider>
    </QueryClientProvider>,
  )
  return { router, queryClient }
}
