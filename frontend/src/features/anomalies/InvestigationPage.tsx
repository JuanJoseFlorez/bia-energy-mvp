import { Sparkles } from 'lucide-react'
import { Link, useParams } from 'react-router'

import { isApiError } from '../../api/client'
import { useAnomalies, useAnomaly, useReadings } from '../../api/queries'
import type { AnomalyDetail } from '../../api/types'
import { Badge } from '../../components/Badge'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { ErrorState } from '../../components/ErrorState'
import { Spinner } from '../../components/Spinner'
import { TONE_FILL } from '../../components/tone'
import { formatConfidence, formatDataDateTime } from '../../lib/format'
import { anomalyStatusMeta, anomalyTypeMeta, confidenceLabel, severityMeta, sourceMeta } from '../../lib/labels'
import { ConsumptionChart } from '../meters/ConsumptionChart'
import { ElectricalCharts } from '../meters/ElectricalCharts'
import { ChangedVarsTable } from './ChangedVarsTable'
import { EvidenceList } from './EvidenceList'
import { StatusActions } from './StatusActions'

function BaselineComparison({ anomaly }: { anomaly: AnomalyDetail }) {
  const readings = useReadings(anomaly.meter_id, anomaly.readings_window)
  const ev = anomaly.evidence
  if (!anomaly.readings_window) return <p className="text-sm text-muted">Sin lecturas para este medidor.</p>
  if (readings.isPending) return <Spinner />
  if (readings.isError) return <ErrorState error={readings.error} onRetry={() => void readings.refetch()} />
  return (
    <>
      <ConsumptionChart
        readings={readings.data.items}
        mode="hourly"
        baselineKwh={ev?.baseline_kwh ?? null}
        changeStart={ev?.change_start ?? null}
        outlierTimestamps={ev?.outlier_timestamps ?? []}
        events={anomaly.related_events}
        height={240}
      />
      <p className="mt-3 text-xs text-muted">
        {formatDataDateTime(anomaly.readings_window.from)} → {formatDataDateTime(anomaly.readings_window.to)} · consumo por hora frente al baseline diario / 24 · horas en UTC ·{' '}
        <Link to={`/meters/${anomaly.meter_id}`} className="text-primary hover:underline">
          Ver medidor
        </Link>
      </p>
    </>
  )
}

/** V / I / FP inside the readings window: the evidence for data-quality and electrical findings. */
function ElectricalInWindow({ anomaly }: { anomaly: AnomalyDetail }) {
  const readings = useReadings(anomaly.meter_id, anomaly.readings_window)
  if (!anomaly.readings_window || !readings.data) return null
  return <ElectricalCharts readings={readings.data.items} changeStart={anomaly.evidence?.change_start ?? null} />
}

function Decision({ anomaly }: { anomaly: AnomalyDetail }) {
  const run = useAnomalies({ analysis_id: anomaly.analysis_id })
  const severity = severityMeta(anomaly.severity)
  const confidence = anomaly.confidence
  return (
    <div className="flex flex-col gap-6 xl:sticky xl:top-0">
      <Card title="Severidad y confianza">
        <div className="flex items-center gap-3">
          <Badge tone={severity.tone}>{severity.label}</Badge>
          {confidence !== null && (
            <span className="text-sm">
              Confianza <strong>{confidenceLabel(confidence)}</strong> · {formatConfidence(confidence)}
            </span>
          )}
        </div>
        {confidence !== null && (
          <div className="mt-3 h-2 rounded bg-surface-2">
            <div className={`h-2 rounded ${TONE_FILL.primary}`} style={{ width: `${Math.round(confidence * 100)}%` }} />
          </div>
        )}
        {anomaly.priority !== null && (
          <p className="mt-3 text-sm text-muted">
            Prioridad {anomaly.priority}
            {run.data ? ` de ${run.data.total}` : ''}
          </p>
        )}
      </Card>
      <Card title="Acción recomendada">
        <p className="mb-4 font-medium">{anomaly.recommended_action ?? '—'}</p>
        <StatusActions key={anomaly.status} anomaly={anomaly} />
      </Card>
      <Card title="Eventos relacionados">
        {anomaly.related_events.length === 0 ? (
          <p className="text-sm text-muted">Sin eventos relacionados.</p>
        ) : (
          <ul className="flex flex-col gap-3 text-sm">
            {anomaly.related_events.map((e) => (
              <li key={e.id}>
                <span className="font-medium">{formatDataDateTime(e.timestamp)}</span> · <Badge tone="warning">{e.type ?? 'Evento'}</Badge>
                {e.description && <p className="mt-1 text-muted">{e.description}</p>}
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  )
}

/** Investigation (brief §12): left, why the AI concluded this; right, what to do about it. */
export function InvestigationPage() {
  const { id = '' } = useParams()
  const anomalyId = /^\d+$/.test(id) ? Number(id) : null
  const query = useAnomaly(anomalyId)

  if (anomalyId === null || (query.isError && isApiError(query.error, 404))) {
    return <EmptyState title="Anomalía no encontrada" description="Puede que pertenezca a un análisis que ya no existe." action={<Link to="/anomalies" className="text-primary hover:underline">Volver a Anomalías IA</Link>} />
  }
  if (query.isPending) return <Spinner />
  if (query.isError) return <ErrorState error={query.error} onRetry={() => void query.refetch()} />

  const a = query.data
  const type = anomalyTypeMeta(a.type)
  const severity = severityMeta(a.severity)
  const status = anomalyStatusMeta(a.status)
  const source = sourceMeta(a.explanation_source)
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-semibold">{a.meter_id}</h1>
        <span className="text-lg text-muted">·</span>
        <span className="text-lg">{type.label}</span>
        <Badge tone={severity.tone}>{severity.label}</Badge>
        <span className="ml-auto flex items-center gap-2 text-sm text-muted">
          Estado <Badge tone={status.tone}>{status.label}</Badge>
        </span>
      </div>

      <div className="grid gap-6 xl:grid-cols-3">
        <div className="flex flex-col gap-6 xl:col-span-2">
          <Card title={<span className="flex items-center gap-2"><Sparkles className="size-4 text-accent" aria-hidden />Qué encontró la IA</span>} action={<Badge tone={source.tone}>{source.label}</Badge>}>
            {a.reason && <p className="text-base font-semibold">{a.reason}</p>}
            {a.explanation && <p className="mt-3 text-sm leading-relaxed text-muted">{a.explanation}</p>}
          </Card>
          <Card title="Comparación contra baseline">
            <BaselineComparison anomaly={a} />
          </Card>
          {a.evidence && (
            <>
              <Card title="Variables que cambiaron">
                <ChangedVarsTable evidence={a.evidence} />
              </Card>
              <ElectricalInWindow anomaly={a} />
              <Card title="Evidencia">
                <EvidenceList evidence={a.evidence} />
              </Card>
            </>
          )}
        </div>
        <Decision anomaly={a} />
      </div>
    </div>
  )
}
