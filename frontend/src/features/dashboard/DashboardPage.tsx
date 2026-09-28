import { Sparkles } from 'lucide-react'

import { useAnomalies, useDashboard } from '../../api/queries'
import { Badge } from '../../components/Badge'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { ErrorState } from '../../components/ErrorState'
import { KpiCard } from '../../components/KpiCard'
import { Spinner } from '../../components/Spinner'
import { formatCompactKwh, formatConfidence, formatLocalDateTime, formatPeriod } from '../../lib/format'
import { runStatusMeta } from '../../lib/labels'
import { RunAnalysisButton } from '../../layout/RunAnalysisButton'
import { PriorityList } from './PriorityList'
import { TypeBreakdown } from './TypeBreakdown'

export function DashboardPage() {
  const dashboard = useDashboard()
  const stats = dashboard.data?.anomalies ?? null
  const anomalies = useAnomalies({}, { enabled: stats !== null })

  if (dashboard.isPending) return <Spinner />
  if (dashboard.isError) return <ErrorState error={dashboard.error} onRetry={() => void dashboard.refetch()} />

  const d = dashboard.data
  const last = d.last_analysis
  const lastMeta = last ? runStatusMeta(last.status) : null
  const lastWhen = last ? (last.finished_at ?? last.started_at) : null

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold">Dashboard</h1>
        <p className="mt-1 text-sm text-muted">
          {d.period ? `${formatPeriod(d.period.from, d.period.to)} · ` : ''}
          {d.meters_count} medidores
        </p>
      </div>

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-3 2xl:grid-cols-6">
        <KpiCard label="Medidores" value={d.meters_count} />
        <KpiCard label="Consumo del periodo" value={formatCompactKwh(d.total_consumption_kwh)} />
        <KpiCard label="Anomalías IA" value={stats ? stats.detected : '—'} footer={stats ? 'detectadas en el último análisis' : undefined} />
        <KpiCard label="Alta prioridad" value={stats ? stats.high_priority : '—'} tone={stats && stats.high_priority > 0 ? 'danger' : undefined} />
        <KpiCard label="Confianza IA" value={stats?.avg_confidence != null ? formatConfidence(stats.avg_confidence) : '—'} footer={stats ? 'promedio del análisis' : undefined} />
        <KpiCard
          label="Último análisis"
          value={<span className="text-lg">{lastWhen ? formatLocalDateTime(lastWhen) : '—'}</span>}
          footer={lastMeta ? <Badge tone={lastMeta.tone}>{lastMeta.label}</Badge> : 'Sin ejecutar'}
        />
      </div>

      {stats === null ? (
        <Card>
          <EmptyState
            icon={<Sparkles className="size-8" aria-hidden />}
            title="Aún no hay análisis"
            description="Ejecuta el análisis IA para detectar, explicar y priorizar las anomalías de los medidores."
            action={<RunAnalysisButton />}
          />
        </Card>
      ) : (
        <div className="grid gap-6 xl:grid-cols-3">
          <Card title="Qué investigar primero" className="xl:col-span-2">
            {anomalies.isPending ? (
              <Spinner />
            ) : anomalies.isError ? (
              <ErrorState error={anomalies.error} onRetry={() => void anomalies.refetch()} />
            ) : anomalies.data.items.length === 0 ? (
              <EmptyState title="Sin anomalías" description="El último análisis no encontró anomalías." />
            ) : (
              <PriorityList items={anomalies.data.items} />
            )}
          </Card>
          <Card title="Por tipo">
            <TypeBreakdown byType={stats.by_type} />
          </Card>
        </div>
      )}
    </div>
  )
}
