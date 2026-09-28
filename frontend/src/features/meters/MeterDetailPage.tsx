import { ChevronRight } from 'lucide-react'
import { useState } from 'react'
import { Link, useParams } from 'react-router'

import { isApiError } from '../../api/client'
import { useAnomaly, useMeter, useReadings } from '../../api/queries'
import type { MeterDetail } from '../../api/types'
import { Badge } from '../../components/Badge'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { ErrorState } from '../../components/ErrorState'
import { SegmentedControl } from '../../components/SegmentedControl'
import { Spinner } from '../../components/Spinner'
import { formatConfidence, formatDataDateTime, formatKwh, formatSignedPercent } from '../../lib/format'
import { anomalyTypeMeta, confidenceLabel, healthMeta, severityMeta } from '../../lib/labels'
import { ConsumptionChart, type ChartMode } from './ConsumptionChart'
import { ElectricalCharts } from './ElectricalCharts'

const MODES: { value: ChartMode; label: string }[] = [
  { value: 'hourly', label: 'Horario' },
  { value: 'daily', label: 'Diario' },
]

function Header({ meter }: { meter: MeterDetail }) {
  const health = healthMeta(meter.health)
  const m = meter.metrics
  return (
    <div className="flex flex-wrap items-end gap-x-8 gap-y-3">
      <div>
        <h1 className="text-2xl font-semibold">{meter.meter_id}</h1>
        <p className="mt-1 text-sm text-muted">{[meter.name, meter.location].filter(Boolean).join(' · ')}</p>
      </div>
      {m && (
        <>
          <div>
            <div className="text-xs text-muted">Últimas 24 h</div>
            <div className="text-xl font-semibold">{formatKwh(m.current_kwh)}</div>
          </div>
          <div>
            <div className="text-xs text-muted">Baseline diario</div>
            <div className="text-xl font-semibold text-accent">{formatKwh(m.baseline_kwh)}</div>
          </div>
          <div>
            <div className="text-xs text-muted">Variación</div>
            <div className={`text-xl font-semibold ${m.variation_pct > 0 ? 'text-danger' : m.variation_pct < 0 ? 'text-primary' : ''}`}>{formatSignedPercent(m.variation_pct)}</div>
          </div>
        </>
      )}
      <Badge tone={health.tone}>{health.label}</Badge>
    </div>
  )
}

function AnomalyCard({ meter }: { meter: MeterDetail }) {
  const a = meter.anomaly
  if (!a) {
    return (
      <Card title="Anomalía">
        <p className="text-sm text-muted">{meter.health === null ? 'Aún no hay análisis para este medidor.' : 'Sin anomalías en el último análisis.'}</p>
      </Card>
    )
  }
  const type = anomalyTypeMeta(a.type)
  const severity = severityMeta(a.severity)
  return (
    <Card title="Anomalía" action={<Link to={`/anomalies/${a.id}`} className="flex items-center gap-1 text-sm text-primary hover:underline">Investigar <ChevronRight className="size-4" aria-hidden /></Link>}>
      <div className="flex flex-wrap items-center gap-2">
        <Badge tone={type.tone}>{type.label}</Badge>
        <Badge tone={severity.tone}>{severity.label}</Badge>
        {a.confidence !== null && <span className="text-xs text-muted">Confianza {confidenceLabel(a.confidence)} · {formatConfidence(a.confidence)}</span>}
      </div>
      {a.reason && <p className="mt-3 font-medium">{a.reason}</p>}
      {a.recommended_action && <p className="mt-2 text-sm text-muted">Acción recomendada: {a.recommended_action}</p>}
    </Card>
  )
}

/** Meter detail (brief §7): metrics header, consumption history vs baseline, electrical variables, anomaly. */
export function MeterDetailPage() {
  const { meterId = '' } = useParams()
  const meter = useMeter(meterId)
  const readings = useReadings(meterId)
  const anomaly = useAnomaly(meter.data?.anomaly?.id ?? null)
  const [mode, setMode] = useState<ChartMode>('hourly')

  if (meter.isPending) return <Spinner />
  if (meter.isError) {
    if (isApiError(meter.error, 404)) {
      return <EmptyState title="Medidor no encontrado" description={`No existe el medidor ${meterId}.`} action={<Link to="/meters" className="text-primary hover:underline">Volver a Medidores</Link>} />
    }
    return <ErrorState error={meter.error} onRetry={() => void meter.refetch()} />
  }

  const m = meter.data
  const changeStart = m.metrics?.change_start ?? null
  return (
    <div className="flex flex-col gap-6">
      <Header meter={m} />

      <Card
        title="Consumo vs. baseline"
        action={<SegmentedControl label="Agrupación" options={MODES} value={mode} onChange={setMode} />}
      >
        {readings.isPending ? (
          <Spinner />
        ) : readings.isError ? (
          <ErrorState error={readings.error} onRetry={() => void readings.refetch()} />
        ) : (
          <>
            <ConsumptionChart
              readings={readings.data.items}
              mode={mode}
              baselineKwh={m.metrics?.baseline_kwh ?? null}
              changeStart={changeStart}
              outlierTimestamps={anomaly.data?.evidence?.outlier_timestamps ?? []}
              events={m.events}
            />
            <p className="mt-3 text-xs text-muted">
              {mode === 'hourly' ? 'Consumo por hora · baseline diario / 24 · ' : 'Consumo por día · '}
              {changeStart ? `cambio desde ${formatDataDateTime(changeStart)} (zona sombreada) · ` : ''}
              {mode === 'hourly' && (anomaly.data?.evidence?.outlier_timestamps.length ?? 0) > 0 ? 'puntos rojos: lecturas fuera de lo esperado · ' : ''}
              horas en UTC
            </p>
          </>
        )}
      </Card>

      {readings.data && <ElectricalCharts readings={readings.data.items} changeStart={changeStart} />}

      <div className="grid gap-6 lg:grid-cols-2">
        <AnomalyCard meter={m} />
        <Card title="Eventos">
          {m.events.length === 0 ? (
            <p className="text-sm text-muted">Sin eventos registrados.</p>
          ) : (
            <ul className="flex flex-col gap-3 text-sm">
              {m.events.map((e) => (
                <li key={e.id}>
                  <span className="font-medium">{formatDataDateTime(e.timestamp)}</span> · <Badge tone="warning">{e.type ?? 'Evento'}</Badge>
                  {e.description && <p className="mt-1 text-muted">{e.description}</p>}
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>
    </div>
  )
}
