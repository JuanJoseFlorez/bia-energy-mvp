import { useMatches } from 'react-router'

import { isActiveRun, useLatestAnalysis } from '../api/queries'
import { Badge } from '../components/Badge'
import { useAnalysis } from '../features/analysis/analysisContext'
import { formatLocalDateTime } from '../lib/format'
import { runStatusMeta } from '../lib/labels'
import type { RouteHandle } from './crumbs'
import { RunAnalysisButton } from './RunAnalysisButton'

function Breadcrumb() {
  const crumbs = useMatches()
    .map((m) => {
      const crumb = (m.handle as RouteHandle | undefined)?.crumb
      return typeof crumb === 'function' ? crumb(m.params) : crumb
    })
    .filter((c): c is string => Boolean(c))
  return (
    <nav aria-label="Ruta" className="flex items-center gap-2 text-sm">
      {crumbs.map((c, i) => (
        <span key={i} className={i === crumbs.length - 1 ? 'font-medium text-text' : 'text-muted'}>
          {i > 0 && <span className="mr-2 text-muted">/</span>}
          {c}
        </span>
      ))}
    </nav>
  )
}

function LastAnalysisPill() {
  const { run } = useAnalysis()
  const fetched = useLatestAnalysis().data
  // The run being tracked is fresher than the cached latest run while it is in progress.
  const latest = isActiveRun(run) ? run : fetched
  if (!latest) return <Badge tone="neutral">Sin análisis</Badge>
  const meta = runStatusMeta(latest.status)
  const when = latest.finished_at ?? latest.started_at
  return (
    <Badge tone={meta.tone}>
      Último análisis{when ? ` · ${formatLocalDateTime(when)}` : ''} · {meta.label}
    </Badge>
  )
}

export function Header() {
  return (
    <header className="flex h-16 shrink-0 items-center justify-between gap-4 border-b border-border px-8">
      <Breadcrumb />
      <div className="flex items-center gap-3">
        <LastAnalysisPill />
        <RunAnalysisButton />
      </div>
    </header>
  )
}
