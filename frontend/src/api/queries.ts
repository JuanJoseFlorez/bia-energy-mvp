import { keepPreviousData, QueryClient, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { apiFetch, isApiError } from './client'
import type {
  AnalysisRun,
  AnomalyDetail,
  AnomalyList,
  AnomalyStatus,
  AnomalyType,
  DashboardSummary,
  ListResult,
  Meter,
  MeterDetail,
  ReadingsResult,
  Session,
  Severity,
} from './types'

export interface AnomalyFilters {
  analysis_id?: number
  type?: AnomalyType
  severity?: Severity
  status?: AnomalyStatus
}

export interface MeterListParams {
  status?: 'all' | 'ok' | 'alert' | 'critical'
  q?: string
  sort?: 'meter_id' | 'consumption' | 'variation' | 'severity'
  order?: 'asc' | 'desc'
}

export interface ReadingsWindow {
  from: string
  to: string
}

export const queryKeys = {
  dashboard: ['dashboard'] as const,
  meters: (params: MeterListParams) => ['meters', params] as const,
  meter: (meterId: string) => ['meter', meterId] as const,
  readings: (meterId: string, window: ReadingsWindow | null) => ['readings', meterId, window?.from ?? null, window?.to ?? null] as const,
  anomalies: (filters: AnomalyFilters) => ['anomalies', filters] as const,
  anomaly: (id: number) => ['anomaly', id] as const,
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

export function useMeters(params: MeterListParams) {
  return useQuery({
    queryKey: queryKeys.meters(params),
    queryFn: ({ signal }) => apiFetch<ListResult<Meter>>(`/meters${toQueryString(params)}`, { signal }),
    placeholderData: keepPreviousData, // keep the table while a new filter loads
  })
}

export function useMeter(meterId: string) {
  return useQuery({
    queryKey: queryKeys.meter(meterId),
    queryFn: ({ signal }) => apiFetch<MeterDetail>(`/meters/${encodeURIComponent(meterId)}`, { signal }),
  })
}

/** Hourly readings of a meter, the whole period or only the given window. */
export function useReadings(meterId: string | null, window: ReadingsWindow | null = null) {
  return useQuery({
    queryKey: queryKeys.readings(meterId ?? '', window),
    queryFn: ({ signal }) =>
      apiFetch<ReadingsResult>(`/meters/${encodeURIComponent(meterId ?? '')}/readings${toQueryString(window ?? {})}`, { signal }),
    enabled: meterId !== null,
  })
}

export function useAnomaly(id: number | null) {
  return useQuery({
    queryKey: queryKeys.anomaly(id ?? 0),
    queryFn: ({ signal }) => apiFetch<AnomalyDetail>(`/anomalies/${id}`, { signal }),
    enabled: id !== null,
  })
}

/** Moves an anomaly along the action workflow and refreshes everything that shows its status. */
export function useUpdateAnomalyStatus(id: number) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (status: AnomalyStatus) => apiFetch<AnomalyDetail>(`/anomalies/${id}`, { method: 'PATCH', body: { status } }),
    onSuccess: (detail) => {
      queryClient.setQueryData(queryKeys.anomaly(id), detail)
      for (const key of ['anomalies', 'meter', 'meters', 'dashboard']) void queryClient.invalidateQueries({ queryKey: [key] })
    },
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
