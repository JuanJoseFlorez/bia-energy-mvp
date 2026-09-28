import { vi } from 'vitest'

export interface MockResponse {
  status?: number
  body?: unknown
}

/** A fixed response, a sequence (the last one repeats) or a function of the request body. */
export type Route = MockResponse | MockResponse[] | ((body: unknown) => MockResponse)

export interface Call {
  method: string
  path: string
  body: unknown
}

/**
 * Stubs global fetch. Routes are keyed "METHOD /path?query"; a key without a query string also
 * matches any query. Unknown routes answer 404 with the backend error shape. Returns the calls made.
 */
export function mockApi(routes: Record<string, Route>): Call[] {
  const calls: Call[] = []
  const served = new Map<string, number>()

  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: string, init: RequestInit = {}) => {
      const url = new URL(input)
      const method = init.method ?? 'GET'
      const body = typeof init.body === 'string' ? JSON.parse(init.body) : undefined
      calls.push({ method, path: url.pathname + url.search, body })

      const key = [`${method} ${url.pathname}${url.search}`, `${method} ${url.pathname}`].find((k) => k in routes)
      let res: MockResponse = { status: 404, body: { error: { code: 'not_found', message: 'not found' } } }
      if (key) {
        const route = routes[key]
        if (typeof route === 'function') res = route(body)
        else if (Array.isArray(route)) {
          const i = served.get(key) ?? 0
          served.set(key, i + 1)
          res = route[Math.min(i, route.length - 1)]
        } else res = route
      }
      return new Response(JSON.stringify(res.body ?? {}), {
        status: res.status ?? 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }),
  )
  return calls
}
