import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { describe, expect, it } from 'vitest'

import { createQueryClient, queryKeys } from '../../api/queries'
import { AnalysisBanner } from '../../layout/AnalysisBanner'
import { RunAnalysisButton } from '../../layout/RunAnalysisButton'
import { runAt } from '../../test/fixtures'
import { mockApi } from '../../test/mockApi'
import { AnalysisProvider } from './AnalysisProvider'

const noLatest = { status: 404, body: { error: { code: 'not_found', message: 'no analysis run yet: not found' } } }

function renderRun() {
  const queryClient = createQueryClient()
  queryClient.setDefaultOptions({ queries: { retry: false } })
  queryClient.setQueryData(queryKeys.dashboard, { stale: true })
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <AnalysisProvider pollIntervalMs={10}>
          <RunAnalysisButton />
          <AnalysisBanner />
        </AnalysisProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  )
  return queryClient
}

describe('analysis run', () => {
  it('shows the steps while running and the summary when completed', async () => {
    mockApi({
      'GET /ai/analysis/latest': noLatest,
      'POST /ai/analyze': { status: 202, body: runAt('PENDING', null) },
      'GET /ai/analysis/8': [
        { body: runAt('RUNNING', 'CORRELATION') },
        { body: runAt('RUNNING', 'CORRELATION') },
        { body: runAt('COMPLETED', 'RECOMMENDATION') },
      ],
    })
    const queryClient = renderRun()
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: /Run AI Analysis/ }))

    const banner = await screen.findByRole('status', { name: 'Análisis IA en curso' })
    await waitFor(() => expect(banner).toHaveTextContent('Paso 4 de 7'))
    expect(screen.getByText('Correlación').closest('[data-state]')).toHaveAttribute('data-state', 'current')
    expect(screen.getByText('Detección').closest('[data-state]')).toHaveAttribute('data-state', 'done')
    expect(screen.getByRole('button', { name: /Analizando/ })).toBeDisabled()

    expect(await screen.findByText('4 anomalías detectadas · 2 requieren atención prioritaria')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Ver anomalías/ })).toHaveAttribute('href', '/anomalies')
    expect(screen.getByRole('button', { name: /Run AI Analysis/ })).toBeEnabled()
    await waitFor(() => expect(queryClient.getQueryState(queryKeys.dashboard)?.isInvalidated).toBe(true))
  })

  it('shows the error and retries when the run fails', async () => {
    const calls = mockApi({
      'GET /ai/analysis/latest': noLatest,
      'POST /ai/analyze': { status: 202, body: runAt('PENDING', null) },
      'GET /ai/analysis/8': { body: runAt('FAILED', 'READINGS') },
    })
    renderRun()
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: /Run AI Analysis/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent('El análisis falló: analysis engine unavailable')

    await user.click(screen.getByRole('button', { name: 'Reintentar' }))
    await waitFor(() => expect(calls.filter((c) => c.method === 'POST')).toHaveLength(2))
  })

  it('follows the active run when another one is already running (409)', async () => {
    mockApi({
      'GET /ai/analysis/latest': [noLatest, { body: runAt('RUNNING', 'EVENTS') }],
      'POST /ai/analyze': { status: 409, body: { error: { code: 'conflict', message: 'an analysis is already running: conflict' } } },
      'GET /ai/analysis/8': { body: runAt('RUNNING', 'EVENTS') },
    })
    renderRun()
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: /Run AI Analysis/ }))

    const banner = await screen.findByRole('status', { name: 'Análisis IA en curso' })
    await waitFor(() => expect(banner).toHaveTextContent('Paso 5 de 7'))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows a start error other than 409', async () => {
    mockApi({
      'GET /ai/analysis/latest': noLatest,
      'POST /ai/analyze': { status: 500, body: { error: { code: 'internal_error', message: 'internal server error' } } },
    })
    renderRun()
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: /Run AI Analysis/ }))

    expect(await screen.findByRole('alert')).toHaveTextContent('No se pudo iniciar el análisis: internal server error')
  })

  it('resumes a run that is already active on load', async () => {
    const calls = mockApi({
      'GET /ai/analysis/latest': { body: runAt('RUNNING', 'BASELINE') },
      'GET /ai/analysis/8': [{ body: runAt('RUNNING', 'DETECTION') }, { body: runAt('COMPLETED', 'RECOMMENDATION') }],
    })
    renderRun()

    expect(await screen.findByText(/4 anomalías detectadas/)).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'POST')).toBe(false)
  })
})
