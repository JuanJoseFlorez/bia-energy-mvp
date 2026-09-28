import { ChevronRight, Sparkles } from 'lucide-react'
import { Link, useNavigate, useSearchParams } from 'react-router'

import { useAnomalies, type AnomalyFilters } from '../../api/queries'
import type { AnomalyStatus, AnomalyType, Severity } from '../../api/types'
import { Badge } from '../../components/Badge'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { ErrorState } from '../../components/ErrorState'
import { Select } from '../../components/Select'
import { Spinner } from '../../components/Spinner'
import { TABLE, TBODY_ROW, TD, TH, THEAD_ROW } from '../../components/table'
import { formatConfidence } from '../../lib/format'
import { anomalyStatusMeta, anomalyTypeMeta, confidenceLabel, severityMeta } from '../../lib/labels'
import { RunAnalysisButton } from '../../layout/RunAnalysisButton'

const TYPES: AnomalyType[] = ['REAL_ANOMALY', 'DATA_QUALITY', 'EXPLAINABLE_ANOMALY', 'FALSE_POSITIVE']
const SEVERITIES: Severity[] = ['HIGH', 'MEDIUM', 'LOW']
const STATUSES: AnomalyStatus[] = ['PENDING', 'INVESTIGATING', 'VALIDATED', 'RESOLVED', 'DISMISSED']

const ALL = { value: '', label: 'Todos' }

function oneOf<T extends string>(value: string | null, allowed: T[]): T | undefined {
  return allowed.find((a) => a === value)
}

/** AI anomalies (brief §11) in investigation order, with the recommended action per type. */
export function AnomaliesPage() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const filters: AnomalyFilters = {
    type: oneOf(searchParams.get('type'), TYPES),
    severity: oneOf(searchParams.get('severity'), SEVERITIES),
    status: oneOf(searchParams.get('status'), STATUSES),
  }
  const anomalies = useAnomalies(filters)

  function setFilter(key: 'type' | 'severity' | 'status', value: string) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value) next.set(key, value)
        else next.delete(key)
        return next
      },
      { replace: true },
    )
  }

  const data = anomalies.data
  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold">Anomalías IA</h1>
        <p className="mt-1 text-sm text-muted">{data?.analysis_id ? `Análisis #${data.analysis_id} · ${data.total} ${data.total === 1 ? 'anomalía' : 'anomalías'} · ordenadas por prioridad` : 'Ordenadas por prioridad de investigación'}</p>
      </div>

      <div className="flex flex-wrap gap-4">
        <Select label="Tipo" value={filters.type ?? ''} onChange={(v) => setFilter('type', v)} options={[ALL, ...TYPES.map((t) => ({ value: t, label: anomalyTypeMeta(t).label }))]} />
        <Select label="Severidad" value={filters.severity ?? ''} onChange={(v) => setFilter('severity', v)} options={[ALL, ...SEVERITIES.map((s) => ({ value: s, label: severityMeta(s).label }))]} />
        <Select label="Estado" value={filters.status ?? ''} onChange={(v) => setFilter('status', v)} options={[ALL, ...STATUSES.map((s) => ({ value: s, label: anomalyStatusMeta(s).label }))]} />
      </div>

      <Card className="overflow-hidden [&>div]:p-0">
        {anomalies.isPending ? (
          <Spinner />
        ) : anomalies.isError ? (
          <ErrorState error={anomalies.error} onRetry={() => void anomalies.refetch()} />
        ) : data!.analysis_id === null ? (
          <EmptyState icon={<Sparkles className="size-8" aria-hidden />} title="Aún no hay análisis" description="Ejecuta el análisis IA para ver las anomalías priorizadas." action={<RunAnalysisButton />} />
        ) : data!.items.length === 0 ? (
          <EmptyState title="Sin anomalías" description="Ninguna anomalía coincide con los filtros." />
        ) : (
          <table className={TABLE}>
            <thead>
              <tr className={THEAD_ROW}>
                <th className={TH}>#</th>
                <th className={TH}>Medidor</th>
                <th className={TH}>Tipo</th>
                <th className={TH}>Severidad</th>
                <th className={TH}>Confianza</th>
                <th className={TH}>Acción recomendada</th>
                <th className={TH}>Estado</th>
                <th className={TH}>
                  <span className="sr-only">Acción</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {data!.items.map((a) => {
                const type = anomalyTypeMeta(a.type)
                const severity = severityMeta(a.severity)
                const status = anomalyStatusMeta(a.status)
                return (
                  <tr key={a.id} onClick={() => navigate(`/anomalies/${a.id}`)} className={`${TBODY_ROW} cursor-pointer`}>
                    <td className={`${TD} text-muted`}>{a.priority ?? '—'}</td>
                    <td className={`${TD} font-semibold`}>{a.meter_id}</td>
                    <td className={TD}>
                      <Badge tone={type.tone}>{type.label}</Badge>
                    </td>
                    <td className={TD}>
                      <Badge tone={severity.tone}>{severity.label}</Badge>
                    </td>
                    <td className={`${TD} whitespace-nowrap`}>{a.confidence === null ? '—' : `${confidenceLabel(a.confidence)} · ${formatConfidence(a.confidence)}`}</td>
                    <td className={`${TD} max-w-xs truncate text-muted`} title={a.recommended_action ?? undefined}>
                      {a.recommended_action ?? '—'}
                    </td>
                    <td className={TD}>
                      <Badge tone={status.tone}>{status.label}</Badge>
                    </td>
                    <td className={`${TD} text-right`}>
                      <Link to={`/anomalies/${a.id}`} onClick={(e) => e.stopPropagation()} className={`inline-flex items-center gap-1 whitespace-nowrap ${a.anomaly ? 'text-primary' : 'text-muted'} hover:underline`}>
                        {type.action}
                        <ChevronRight className="size-4" aria-hidden />
                      </Link>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  )
}
