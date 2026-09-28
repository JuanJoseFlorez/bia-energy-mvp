import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { emptyDashboard } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { demoSession, renderApp } from '../../test/renderApp'
import { SESSION_KEY } from './AuthProvider'

const shellRoutes = {
  'GET /dashboard/summary': { body: emptyDashboard },
  'GET /ai/analysis/latest': { status: 404, body: { error: { code: 'not_found', message: 'no analysis run yet: not found' } } },
}

describe('auth', () => {
  it('redirects to /login without a session', async () => {
    mockApi(shellRoutes)
    const { router } = renderApp('/')

    expect(await screen.findByRole('heading', { name: 'Ingresa a tu cuenta' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/login')
  })

  it('signs in, stores the session and returns to the requested page', async () => {
    const calls = mockApi({ ...shellRoutes, 'POST /auth/login': { body: demoSession } })
    const { router } = renderApp('/anomalies')
    const user = userEvent.setup()

    await user.type(await screen.findByLabelText('Usuario'), 'demo')
    await user.type(screen.getByLabelText('Contraseña'), 'demo')
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/anomalies'))
    expect(JSON.parse(localStorage.getItem(SESSION_KEY)!)).toEqual(demoSession)
    expect(calls.find((c) => c.path === '/auth/login')?.body).toEqual({ username: 'demo', password: 'demo' })
  })

  it('shows a Spanish message on wrong credentials', async () => {
    mockApi({ 'POST /auth/login': { status: 401, body: { error: { code: 'unauthorized', message: 'invalid username or password: unauthorized' } } } })
    renderApp('/login')
    const user = userEvent.setup()

    await user.type(screen.getByLabelText('Usuario'), 'demo')
    await user.type(screen.getByLabelText('Contraseña'), 'wrong')
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Usuario o contraseña incorrectos')
    expect(localStorage.getItem(SESSION_KEY)).toBeNull()
  })

  it('signs out from the sidebar', async () => {
    mockApi(shellRoutes)
    const { router } = renderApp('/', { session: demoSession })
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: /Cerrar sesión/ }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(localStorage.getItem(SESSION_KEY)).toBeNull()
  })

  it('skips the login page when already signed in', async () => {
    mockApi(shellRoutes)
    const { router } = renderApp('/login', { session: demoSession })

    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
  })
})
