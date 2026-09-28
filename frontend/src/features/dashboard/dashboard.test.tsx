import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { analyzedDashboard, anomalyList, emptyDashboard } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { demoSession, renderApp } from '../../test/renderApp'

const noLatest = { status: 404, body: { error: { code: 'not_found', message: 'no analysis run yet: not found' } } }

describe('dashboard', () => {
  it('prompts to run the analysis before the first run', async () => {
    const calls = mockApi({ 'GET /dashboard/summary': { body: emptyDashboard }, 'GET /ai/analysis/latest': noLatest })
    renderApp('/', { session: demoSession })

    expect(await screen.findByText('Aún no hay análisis')).toBeInTheDocument()
    expect(screen.getByText('1 sep – 14 sep 2026 · 12 medidores')).toBeInTheDocument()
    expect(screen.getByText('155,3 K kWh')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /Run AI Analysis/ })).toHaveLength(2)
    expect(calls.some((c) => c.path.startsWith('/anomalies'))).toBe(false)
  })

  it('shows the KPIs and the anomalies in priority order with the §11 actions', async () => {
    mockApi({
      'GET /dashboard/summary': { body: analyzedDashboard },
      'GET /ai/analysis/latest': { body: { id: 7, status: 'COMPLETED', current_step: 'RECOMMENDATION', started_at: '2026-09-27T20:04:00Z', updated_at: '2026-09-27T20:04:03Z', finished_at: '2026-09-27T20:04:03Z', summary: { anomalies_detected: 4, high_priority: 2 }, error: null } },
      'GET /anomalies': { body: anomalyList },
    })
    renderApp('/', { session: demoSession })

    const rows = await screen.findAllByRole('link', { name: /M-1\d\d/ })
    expect(rows.map((r) => within(r).getByText(/^M-1/).textContent)).toEqual(['M-109', 'M-112', 'M-104', 'M-106'])
    expect(rows[0]).toHaveAttribute('href', '/anomalies/31')
    expect(within(rows[0]).getByText('Investigar')).toBeInTheDocument()
    expect(within(rows[1]).getByText('Validar')).toBeInTheDocument()
    expect(within(rows[2]).getByText('Validar operación')).toBeInTheDocument()
    expect(within(rows[3]).getByText('No escalar')).toBeInTheDocument()
    expect(screen.getByText('92 %')).toBeInTheDocument()
    expect(screen.getByLabelText('2 de alta prioridad')).toBeInTheDocument()
  })
})
