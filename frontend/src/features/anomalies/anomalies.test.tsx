import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { anomalyList, emptyDashboard } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { demoSession, renderApp } from '../../test/renderApp'
import { anomalyDetail, readings } from '../../test/screenFixtures'

const shell = {
  'GET /dashboard/summary': { body: emptyDashboard },
  'GET /ai/analysis/latest': { status: 404, body: { error: { code: 'not_found', message: 'no analysis run yet: not found' } } },
}

const investigation = {
  ...shell,
  'GET /anomalies/31': { body: anomalyDetail },
  'GET /anomalies': { body: anomalyList },
  'GET /meters/M-109/readings': { body: readings },
}

describe('anomalies list', () => {
  it('lists anomalies in priority order with the §11 action per type', async () => {
    mockApi({ ...shell, 'GET /anomalies': { body: anomalyList } })
    renderApp('/anomalies', { session: demoSession })

    const rows = (await screen.findAllByRole('row')).slice(1)
    expect(rows.map((r) => within(r).getAllByRole('cell')[1].textContent)).toEqual(['M-109', 'M-112', 'M-104', 'M-106'])
    expect(rows.map((r) => within(r).getByRole('link').textContent)).toEqual(['Investigar', 'Validar', 'Validar operación', 'No escalar'])
    expect(within(rows[0]).getByText('Alta · 90 %')).toBeInTheDocument()
    expect(screen.getByText(/Análisis #7 · 4 anomalías/)).toBeInTheDocument()
  })

  it('filters through the URL', async () => {
    const calls = mockApi({ ...shell, 'GET /anomalies': { body: anomalyList } })
    const { router } = renderApp('/anomalies', { session: demoSession })
    const user = userEvent.setup()

    await screen.findAllByRole('row')
    await user.selectOptions(screen.getByLabelText('Tipo'), 'DATA_QUALITY')

    await waitFor(() => expect(router.state.location.search).toBe('?type=DATA_QUALITY'))
    await waitFor(() => expect(calls.some((c) => c.path === '/anomalies?type=DATA_QUALITY')).toBe(true))
  })

  it('prompts to run the analysis before the first run', async () => {
    mockApi({ ...shell, 'GET /anomalies': { body: { analysis_id: null, items: [], total: 0 } } })
    renderApp('/anomalies', { session: demoSession })

    expect(await screen.findByText('Aún no hay análisis')).toBeInTheDocument()
  })
})

describe('investigation', () => {
  it('shows what the AI found, the changed variables and the evidence', async () => {
    const calls = mockApi(investigation)
    renderApp('/anomalies/31', { session: demoSession })

    expect(await screen.findByText('Consumo +109,7 % sobre el baseline sin evento conocido.')).toBeInTheDocument()
    expect(screen.getByText(/pasó de 1.052,7 kWh a 2.207,6 kWh/)).toBeInTheDocument()
    expect(screen.getByText('IA (LLM)')).toBeInTheDocument()
    const current = screen.getByText('Corriente (A)').closest('tr')!
    expect(within(current).getByText('158,2')).toBeInTheDocument()
    expect(within(current).getByText('+100,9 %')).toBeInTheDocument()
    expect(screen.getByText('Sin evento operativo que lo explique')).toBeInTheDocument()
    expect(screen.getByText('Caída del factor de potencia')).toBeInTheDocument()
    expect(screen.getByText('No operational event reported')).toBeInTheDocument()
    expect(await screen.findByText('Prioridad 1 de 4')).toBeInTheDocument()
    await waitFor(() =>
      expect(calls.some((c) => c.path === '/meters/M-109/readings?from=2026-09-05T14%3A00%3A00Z&to=2026-09-14T23%3A00%3A00Z')).toBe(true),
    )
  })

  it('moves the anomaly along the workflow', async () => {
    const calls = mockApi({
      ...investigation,
      'PATCH /anomalies/31': (body) => ({
        body: { ...anomalyDetail, status: (body as { status: string }).status, next_statuses: ['VALIDATED', 'RESOLVED', 'DISMISSED'] },
      }),
    })
    renderApp('/anomalies/31', { session: demoSession })
    const user = userEvent.setup()

    const buttons = await screen.findAllByRole('button', { name: /Marcar en investigación|Validar|Descartar/ })
    expect(buttons.map((b) => b.textContent)).toEqual(['Marcar en investigación', 'Validar', 'Descartar'])

    await user.click(screen.getByRole('button', { name: 'Marcar en investigación' }))

    expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ status: 'INVESTIGATING' })
    expect(await screen.findByRole('button', { name: 'Resolver' })).toBeInTheDocument()
    expect(screen.getAllByText('En investigación').length).toBeGreaterThan(0)
  })

  it('asks before dismissing', async () => {
    const calls = mockApi({ ...investigation, 'PATCH /anomalies/31': { body: { ...anomalyDetail, status: 'DISMISSED', next_statuses: [] } } })
    renderApp('/anomalies/31', { session: demoSession })
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Descartar' }))
    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'Cancelar' }))
    expect(calls.some((c) => c.method === 'PATCH')).toBe(false)

    await user.click(screen.getByRole('button', { name: 'Descartar' }))
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Descartar' }))

    expect(await screen.findByText('Esta anomalía está descartada.')).toBeInTheDocument()
    expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ status: 'DISMISSED' })
  })

  it('shows the backend message when the transition is rejected', async () => {
    mockApi({
      ...investigation,
      'PATCH /anomalies/31': { status: 409, body: { error: { code: 'conflict', message: 'cannot change status from RESOLVED to INVESTIGATING: conflict' } } },
    })
    renderApp('/anomalies/31', { session: demoSession })
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Marcar en investigación' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('No se pudo actualizar: cannot change status from RESOLVED to INVESTIGATING: conflict')
  })

  it('reports an unknown anomaly', async () => {
    mockApi({ ...shell, 'GET /anomalies/999': { status: 404, body: { error: { code: 'not_found', message: 'anomaly 999: not found' } } } })
    renderApp('/anomalies/999', { session: demoSession })

    expect(await screen.findByText('Anomalía no encontrada')).toBeInTheDocument()
  })
})
