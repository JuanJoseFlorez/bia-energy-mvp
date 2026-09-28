import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'

import { ApiError, isApiError } from '../../api/client'
import { fetchLatestAnalysis, isActiveRun, queryKeys, useAnalysisRun, useLatestAnalysis, useStartAnalysis } from '../../api/queries'
import { AnalysisContext } from './analysisContext'

/** Tracks one analysis run for the whole app shell, so its progress survives navigation. */
export function AnalysisProvider({ children, pollIntervalMs = 1000 }: { children: ReactNode; pollIntervalMs?: number }) {
  const queryClient = useQueryClient()
  const latest = useLatestAnalysis()
  const startMutation = useStartAnalysis()
  const [trackedId, setTrackedId] = useState<number | null>(null)
  const [resumeChecked, setResumeChecked] = useState(false)

  // Once, when the latest run first loads: a run already active (e.g. after a page refresh) is resumed.
  // State is adjusted during render (React's pattern for deriving state), not in an effect.
  if (!resumeChecked && latest.isSuccess) {
    setResumeChecked(true)
    if (trackedId === null && isActiveRun(latest.data)) setTrackedId(latest.data!.id)
  }

  const runQuery = useAnalysisRun(trackedId, pollIntervalMs)
  const run = trackedId === null ? null : (runQuery.data ?? null)

  // When the tracked run ends, everything derived from runs is refetched (dashboard, lists, header pill).
  const finished = run !== null && !isActiveRun(run)
  useEffect(() => {
    if (!finished || trackedId === null) return
    const own = queryKeys.analysis(trackedId)
    void queryClient.invalidateQueries({
      predicate: (q) => !(q.queryKey[0] === own[0] && q.queryKey[1] === own[1]),
    })
  }, [finished, trackedId, queryClient])

  const start = useCallback(() => {
    startMutation.mutate(undefined, {
      onSuccess: (created) => {
        queryClient.setQueryData(queryKeys.analysis(created.id), created)
        setTrackedId(created.id)
      },
      onError: async (err) => {
        if (!isApiError(err, 409)) return
        // Another tab or user already started a run: follow that one instead of failing.
        const active = await queryClient.fetchQuery({ queryKey: queryKeys.latestAnalysis, queryFn: ({ signal }) => fetchLatestAnalysis(signal) })
        startMutation.reset()
        if (active) setTrackedId(active.id)
      },
    })
  }, [startMutation, queryClient])

  const dismiss = useCallback(() => {
    startMutation.reset()
    setTrackedId(null)
  }, [startMutation])

  const startError = startMutation.error instanceof ApiError && startMutation.error.status !== 409 ? startMutation.error : null
  const isActive = startMutation.isPending || isActiveRun(run)

  const value = useMemo(() => ({ run, isActive, startError, start, dismiss }), [run, isActive, startError, start, dismiss])
  return <AnalysisContext.Provider value={value}>{children}</AnalysisContext.Provider>
}
