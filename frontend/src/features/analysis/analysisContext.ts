import { createContext, useContext } from 'react'

import type { ApiError } from '../../api/client'
import type { AnalysisRun } from '../../api/types'

export interface AnalysisState {
  /** The run shown in the banner (started here, or found active on load), null when none. */
  run: AnalysisRun | null
  /** A run is starting or PENDING/RUNNING: the Run button is disabled. */
  isActive: boolean
  /** Starting failed for a reason other than "a run is already active". */
  startError: ApiError | null
  start: () => void
  dismiss: () => void
}

export const AnalysisContext = createContext<AnalysisState | null>(null)

export function useAnalysis(): AnalysisState {
  const ctx = useContext(AnalysisContext)
  if (!ctx) throw new Error('useAnalysis must be used inside AnalysisProvider')
  return ctx
}
