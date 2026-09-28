import { Check, TriangleAlert, X } from 'lucide-react'

import type { Evidence } from '../../api/types'
import { formatNumber } from '../../lib/format'
import { evidenceLabel } from '../../lib/labels'

// Signals that argue against a real, unexplained anomaly are shown with ✗.
const NEGATIVE = new Set(['no_explaining_event'])

/** Evidence behind the classification (brief §12): signals, failed data-quality checks and key figures. */
export function EvidenceList({ evidence }: { evidence: Evidence }) {
  return (
    <div className="flex flex-col gap-4 text-sm">
      <ul className="flex flex-col gap-2">
        {evidence.signals.map((s) => (
          <li key={s} className="flex items-start gap-2">
            {NEGATIVE.has(s) ? <X className="mt-0.5 size-4 shrink-0 text-danger" aria-hidden /> : <Check className="mt-0.5 size-4 shrink-0 text-primary" aria-hidden />}
            {evidenceLabel(s)}
          </li>
        ))}
        {evidence.failed_checks.map((c) => (
          <li key={c.check} className="flex items-start gap-2">
            <TriangleAlert className="mt-0.5 size-4 shrink-0 text-warning" aria-hidden />
            {evidenceLabel(c.check)} · {c.hours} h
          </li>
        ))}
      </ul>
      <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-muted">
        <dt>Lecturas fuera de lo esperado</dt>
        <dd className="text-text">{evidence.outlier_timestamps.length}</dd>
        {evidence.effect_size !== null && (
          <>
            <dt>Tamaño del efecto</dt>
            <dd className="text-text">{formatNumber(evidence.effect_size)}</dd>
          </>
        )}
        {evidence.profile_correlation !== null && (
          <>
            <dt>Correlación del perfil horario</dt>
            <dd className="text-text">{formatNumber(evidence.profile_correlation, 2)}</dd>
          </>
        )}
      </dl>
      {!evidence.baseline_reliable && (
        <p className="flex items-center gap-2 text-warning">
          <TriangleAlert className="size-4" aria-hidden />
          Baseline con poca historia previa: la confianza se redujo.
        </p>
      )}
    </div>
  )
}
