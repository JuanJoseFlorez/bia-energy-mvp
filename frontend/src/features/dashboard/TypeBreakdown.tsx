import type { AnomalyType } from '../../api/types'
import { TONE_FILL } from '../../components/tone'
import { anomalyTypeMeta } from '../../lib/labels'

const ORDER: AnomalyType[] = ['REAL_ANOMALY', 'DATA_QUALITY', 'EXPLAINABLE_ANOMALY', 'FALSE_POSITIVE']

export function TypeBreakdown({ byType }: { byType: Record<AnomalyType, number> }) {
  const max = Math.max(1, ...ORDER.map((t) => byType[t] ?? 0))
  return (
    <ul className="flex flex-col gap-4">
      {ORDER.map((t) => {
        const meta = anomalyTypeMeta(t)
        const count = byType[t] ?? 0
        return (
          <li key={t} className="flex flex-col gap-1.5">
            <div className="flex justify-between text-sm">
              <span>{meta.label}</span>
              <span className="font-medium">{count}</span>
            </div>
            <div className="h-2 rounded bg-surface-2">
              <div className={`h-2 rounded ${TONE_FILL[meta.tone]}`} style={{ width: `${(count / max) * 100}%` }} />
            </div>
          </li>
        )
      })}
    </ul>
  )
}
