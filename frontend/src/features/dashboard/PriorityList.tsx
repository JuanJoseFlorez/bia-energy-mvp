import { ChevronRight } from 'lucide-react'
import { Link } from 'react-router'

import type { AnomalyItem } from '../../api/types'
import { Badge } from '../../components/Badge'
import { formatConfidence } from '../../lib/format'
import { anomalyTypeMeta, confidenceLabel, severityMeta } from '../../lib/labels'

/** Anomalies in investigation order — answers "¿cuál debería investigarse primero?". */
export function PriorityList({ items }: { items: AnomalyItem[] }) {
  return (
    <ol className="divide-y divide-border">
      {items.map((a) => {
        const type = anomalyTypeMeta(a.type)
        const severity = severityMeta(a.severity)
        return (
          <li key={a.id}>
            <Link to={`/anomalies/${a.id}`} className="-mx-2 flex items-center gap-4 rounded-lg px-2 py-3 hover:bg-surface-2">
              <span className="w-6 text-center text-lg font-semibold text-muted">{a.priority ?? '—'}</span>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-semibold">{a.meter_id}</span>
                  <Badge tone={type.tone}>{type.label}</Badge>
                  <Badge tone={severity.tone}>{severity.label}</Badge>
                  {a.confidence !== null && (
                    <span className="text-xs text-muted">
                      Confianza {confidenceLabel(a.confidence)} · {formatConfidence(a.confidence)}
                    </span>
                  )}
                </div>
                {a.reason && <p className="mt-1 truncate text-sm text-muted">{a.reason}</p>}
              </div>
              <span className={`flex shrink-0 items-center gap-1 text-sm ${a.anomaly ? 'text-primary' : 'text-muted'}`}>
                {type.action}
                <ChevronRight className="size-4" aria-hidden />
              </span>
            </Link>
          </li>
        )
      })}
    </ol>
  )
}
