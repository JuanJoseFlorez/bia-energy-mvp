import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { emptyDashboard } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { demoSession, renderApp } from '../../test/renderApp'
import { anomalyDetail, meterDetail, meterList, readings } from '../../test/screenFixtures'

const shell = {
  'GET /dashboard/summary': { body: emptyDashboard },
  'GET /ai/analysis/latest': { status: 404, body: { error: { code: 'not_found', message: 'no analysis run yet: not found' } } },
}

const meterRequests = (calls: { path: string }[]) => calls.filter((c) => c.path.startsWith('/meters?') || c.path === '/meters').map((c) => c.path)

describe('meters list', () => {
  it('renders the table and opens a meter on row click', async () => {
    mockApi({ ...shell, 'GET /meters': { body: meterList } })
    const { router } = renderApp('/meters', { session: demoSession })
    const user = userEvent.setup()

    const row = (await screen.findByText('M-109')).closest('tr')!
    expect(within(row).getByText('2.207,6 kWh')).toBeInTheDocument()
    expect(within(row).getByText('+109,7 %')).toHaveClass('text-danger')
    expect(within(row).getByText('Crítico')).toBeInTheDocument()
    expect(within(row).getByText('Alta')).toBeInTheDocument()
    expect(within(screen.getByText('M-101').closest('tr')!).getByText('−1,2 %')).toHaveClass('text-primary')

    await user.click(row)
    await waitFor(() => expect(router.state.location.pathname).toBe('/meters/M-109'))
  })

  it('keeps filter, search and sort in the URL and in the request', async () => {
    const calls = mockApi({ ...shell, 'GET /meters': { body: meterList } })
    const { router } = renderApp('/meters?status=critical', { session: demoSession })
    const user = userEvent.setup()

    await screen.findByText('M-109')
    expect(meterRequests(calls)).toContain('/meters?status=critical&sort=meter_id&order=asc')
    expect(screen.getByRole('button', { name: 'Críticos' })).toHaveAttribute('aria-pressed', 'true')

    await user.click(screen.getByRole('button', { name: 'Alertas' }))
    await waitFor(() => expect(router.state.location.search).toBe('?status=alert'))

    await user.selectOptions(screen.getByLabelText('Ordenar por'), 'variation')
    await waitFor(() => expect(meterRequests(calls)).toContain('/meters?status=alert&sort=variation&order=desc'))

    await user.type(screen.getByLabelText('Buscar por medidor'), 'm-10')
    await waitFor(() => expect(router.state.location.search).toContain('q=m-10'))
    await waitFor(() => expect(meterRequests(calls)).toContain('/meters?status=alert&q=m-10&sort=variation&order=desc'))

    await user.click(screen.getByRole('button', { name: 'Orden descendente' }))
    await waitFor(() => expect(router.state.location.search).toContain('order=asc'))
  })
})

describe('meter detail', () => {
  it('shows the §7 header, the anomaly card and the events', async () => {
    const calls = mockApi({
      ...shell,
      'GET /meters/M-109': { body: meterDetail },
      'GET /meters/M-109/readings': { body: readings },
      'GET /anomalies/31': { body: anomalyDetail },
    })
    renderApp('/meters/M-109', { session: demoSession })

    expect(await screen.findByRole('heading', { name: 'M-109' })).toBeInTheDocument()
    expect(screen.getByText('2.207,6 kWh')).toBeInTheDocument()
    expect(screen.getByText('1.052,7 kWh')).toBeInTheDocument()
    expect(screen.getByText('+109,7 %')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Investigar/ })).toHaveAttribute('href', '/anomalies/31')
    expect(screen.getByText('No operational event reported')).toBeInTheDocument()
    await waitFor(() => expect(calls.some((c) => c.path === '/anomalies/31')).toBe(true))
    expect(await screen.findByText(/puntos rojos/)).toBeInTheDocument()
  })

  it('reports an unknown meter', async () => {
    mockApi({ ...shell, 'GET /meters/M-999': { status: 404, body: { error: { code: 'not_found', message: 'meter M-999: not found' } } } })
    renderApp('/meters/M-999', { session: demoSession })

    expect(await screen.findByText('Medidor no encontrado')).toBeInTheDocument()
  })
})
