import { describe, expect, it, vi } from 'vitest'

import { mockApi } from '../test/mockApi'
import { ApiError, apiFetch } from './client'

describe('apiFetch', () => {
  it('returns the JSON body and sends JSON bodies', async () => {
    const calls = mockApi({ 'POST /auth/login': { body: { token: 't' } } })

    await expect(apiFetch('/auth/login', { method: 'POST', body: { username: 'demo' } })).resolves.toEqual({ token: 't' })
    expect(calls).toEqual([{ method: 'POST', path: '/auth/login', body: { username: 'demo' } }])
  })

  it('turns the backend error body into ApiError', async () => {
    mockApi({ 'GET /x': { status: 409, body: { error: { code: 'conflict', message: 'an analysis is already running: conflict' } } } })

    const err = await apiFetch('/x').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 409, code: 'conflict', message: 'an analysis is already running: conflict' })
  })

  it('keeps a generic message for non-JSON error bodies', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<html>bad gateway</html>', { status: 502 })))

    await expect(apiFetch('/x')).rejects.toMatchObject({ status: 502, code: 'http_error', message: 'Error 502' })
  })

  it('reports network failures with status 0', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('Failed to fetch') }))

    await expect(apiFetch('/x')).rejects.toMatchObject({ status: 0, code: 'network_error' })
  })
})
