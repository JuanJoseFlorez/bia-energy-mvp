import { QueryClient, useMutation, useQuery } from '@tanstack/react-query'

import { apiFetch, isApiError } from './client'
import type { AnalysisRun, AnomalyList, AnomalyType, DashboardSummary, Session, Severity, AnomalyStatus } from './types'

export interface AnomalyFilters {
  analysis_id?: number
  type?: AnomalyType
  severity?: Severity
  status?: AnomalyStatus
}

export const queryKeys = {
  dashboard: ['dashboard'] as const,
  anomalies: (filters: AnomalyFilters) => ['anomalies', filters] as const,
  latestAnalysis: ['analysis', 'latest'] as const,
  analysis: (id: number) => ['analysis', id] as const,
}

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: false,
        // 4xx answers will not change on retry; network/5xx errors get one more try.
        retry: (failures, err) => !(isApiError(err) && err.status >= 400 && err.status < 500) && failures < 1,
      },
    },
  })
}

export function toQueryString(params: object): string {
  const qs = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') qs.set(key, String(value))
  }
  const s = qs.toString()
  return s ? `?${s}` : ''
}

export function isActiveRun(run: AnalysisRun | null | undefined): boolean {
  return run?.status === 'PENDING' || run?.status === 'RUNNING'
}

export function fetchLatestAnalysis(signal?: AbortSignal): Promise<AnalysisRun | null> {
  return apiFetch<AnalysisRun>('/ai/analysis/latest', { signal }).catch((err: unknown) => {
    if (isApiError(err, 404)) return null // no run yet
    throw err
  })
}

export function useDashboard() {
  return useQuery({
    queryKey: queryKeys.dashboard,
    queryFn: ({ signal }) => apiFetch<DashboardSummary>('/dashboard/summary', { signal }),
  })
}

export function useAnomalies(filters: AnomalyFilters, options: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: queryKeys.anomalies(filters),
    queryFn: ({ signal }) => apiFetch<AnomalyList>(`/anomalies${toQueryString(filters)}`, { signal }),
    enabled: options.enabled ?? true,
  })
}

export function useLatestAnalysis() {
  return useQuery({
    queryKey: queryKeys.latestAnalysis,
    queryFn: ({ signal }) => fetchLatestAnalysis(signal),
  })
}

/** Polls one run every pollMs while it is PENDING or RUNNING. */
export function useAnalysisRun(id: number | null, pollMs: number) {
  return useQuery({
    queryKey: queryKeys.analysis(id ?? 0),
    queryFn: ({ signal }) => apiFetch<AnalysisRun>(`/ai/analysis/${id}`, { signal }),
    enabled: id !== null,
    staleTime: 0,
    refetchInterval: (query) => (isActiveRun(query.state.data) ? pollMs : false),
  })
}

export function useStartAnalysis() {
  return useMutation({
    mutationFn: () => apiFetch<AnalysisRun>('/ai/analyze', { method: 'POST' }),
  })
}

export function useLogin() {
  return useMutation({
    mutationFn: (credentials: { username: string; password: string }) =>
      apiFetch<Session>('/auth/login', { method: 'POST', body: credentials }),
  })
}
