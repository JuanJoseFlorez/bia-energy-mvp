import { ArrowDown, ArrowUp, Search } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'

import { useMeters, type MeterListParams } from '../../api/queries'
import { Badge } from '../../components/Badge'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { ErrorState } from '../../components/ErrorState'
import { SegmentedControl } from '../../components/SegmentedControl'
import { Select } from '../../components/Select'
import { Spinner } from '../../components/Spinner'
import { TABLE, TBODY_ROW, TD, TH, THEAD_ROW } from '../../components/table'
import { formatKwh, formatSignedPercent } from '../../lib/format'
import { healthMeta, severityMeta } from '../../lib/labels'

type Status = NonNullable<MeterListParams['status']>
type Sort = NonNullable<MeterListParams['sort']>

const STATUS_OPTIONS: { value: Status; label: string }[] = [
  { value: 'all', label: 'Todos' },
  { value: 'ok', label: 'Normales' },
  { value: 'alert', label: 'Alertas' },
  { value: 'critical', label: 'Críticos' },
]

const SORT_OPTIONS: { value: Sort; label: string }[] = [
  { value: 'meter_id', label: 'Medidor' },
  { value: 'consumption', label: 'Consumo' },
  { value: 'variation', label: 'Variación' },
  { value: 'severity', label: 'Severidad' },
]

const SEARCH_DEBOUNCE_MS = 300

function pick<T extends string>(value: string | null, allowed: { value: T }[], fallback: T): T {
  return allowed.some((o) => o.value === value) ? (value as T) : fallback
}

/** Meters table (brief §6): filters, search and sort live in the URL. */
export function MetersPage() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const status = pick(searchParams.get('status'), STATUS_OPTIONS, 'all')
  const sort = pick(searchParams.get('sort'), SORT_OPTIONS, 'meter_id')
  const order: 'asc' | 'desc' = searchParams.get('order') === 'desc' ? 'desc' : searchParams.get('order') === 'asc' ? 'asc' : sort === 'meter_id' ? 'asc' : 'desc'
  const q = searchParams.get('q') ?? ''
  const [search, setSearch] = useState(q)

  const update = useCallback(
    (changes: Record<string, string | null>) =>
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          for (const [k, v] of Object.entries(changes)) {
            if (v === null || v === '') next.delete(k)
            else next.set(k, v)
          }
          return next
        },
        { replace: true },
      ),
    [setSearchParams],
  )

  // Typing updates the URL (and the request) once the user pauses.
  useEffect(() => {
    if (search === q) return
    const timer = setTimeout(() => update({ q: search.trim() || null }), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [search, q, update])

  const meters = useMeters({ status, q: q || undefined, sort, order })
  const noAnalysis = meters.data?.items.every((m) => m.health === null) ?? false

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold">Medidores</h1>
        <p className="mt-1 text-sm text-muted">Consumo de las últimas 24 h frente a su baseline diario</p>
      </div>

      <div className="flex flex-wrap items-center gap-4">
        <SegmentedControl label="Estado" options={STATUS_OPTIONS} value={status} onChange={(v) => update({ status: v === 'all' ? null : v })} />
        <label className="flex items-center gap-2 rounded-lg border border-border bg-surface px-3 py-1.5 text-sm focus-within:border-primary">
          <Search className="size-4 text-muted" aria-hidden />
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Buscar por medidor" aria-label="Buscar por medidor" className="bg-transparent outline-none placeholder:text-muted" />
        </label>
        <div className="ml-auto flex items-center gap-2">
          <Select label="Ordenar por" options={SORT_OPTIONS} value={sort} onChange={(v) => update({ sort: v === 'meter_id' ? null : v, order: null })} />
          <button
            type="button"
            onClick={() => update({ order: order === 'asc' ? 'desc' : 'asc' })}
            aria-label={order === 'asc' ? 'Orden ascendente' : 'Orden descendente'}
            className="rounded-lg border border-border p-2 text-muted hover:text-text"
          >
            {order === 'asc' ? <ArrowUp className="size-4" aria-hidden /> : <ArrowDown className="size-4" aria-hidden />}
          </button>
        </div>
      </div>

      {noAnalysis && status === 'all' && (
        <p className="text-sm text-muted">Aún no hay análisis: el estado, la variación y la anomalía aparecen después de ejecutar el análisis IA.</p>
      )}

      <Card className="overflow-hidden [&>div]:p-0">
        {meters.isPending ? (
          <Spinner />
        ) : meters.isError ? (
          <ErrorState error={meters.error} onRetry={() => void meters.refetch()} />
        ) : meters.data.items.length === 0 ? (
          <EmptyState title="Sin medidores" description={status === 'all' ? 'Ningún medidor coincide con la búsqueda.' : 'Ningún medidor tiene ese estado en el último análisis.'} />
        ) : (
          <table className={TABLE}>
            <thead>
              <tr className={THEAD_ROW}>
                <th className={TH}>Medidor</th>
                <th className={`${TH} text-right`}>Consumo 24 h</th>
                <th className={`${TH} text-right`}>Variación</th>
                <th className={TH}>Estado</th>
                <th className={TH}>Anomalía</th>
              </tr>
            </thead>
            <tbody>
              {meters.data.items.map((m) => {
                const health = healthMeta(m.health)
                const severity = severityMeta(m.anomaly?.severity ?? null)
                const variation = m.metrics?.variation_pct ?? null
                return (
                  <tr key={m.meter_id} onClick={() => navigate(`/meters/${m.meter_id}`)} className={`${TBODY_ROW} cursor-pointer`}>
                    <td className={TD}>
                      <div className="font-semibold">{m.meter_id}</div>
                      <div className="text-xs text-muted">{[m.name, m.location].filter(Boolean).join(' · ')}</div>
                    </td>
                    <td className={`${TD} text-right`}>{m.metrics ? formatKwh(m.metrics.current_kwh) : '—'}</td>
                    <td className={`${TD} text-right ${variation === null ? '' : variation > 0 ? 'text-danger' : variation < 0 ? 'text-primary' : ''}`}>
                      {variation === null ? '—' : formatSignedPercent(variation)}
                    </td>
                    <td className={TD}>
                      <Badge tone={health.tone}>{health.label}</Badge>
                    </td>
                    <td className={TD}>{m.anomaly ? <Badge tone={severity.tone}>{severity.label}</Badge> : <span className="text-muted">—</span>}</td>
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
