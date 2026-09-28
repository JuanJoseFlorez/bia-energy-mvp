import type { MeterEvent, Reading } from '../../api/types'
import { formatDataDay } from '../../lib/format'

/** One reading on a numeric time axis (ms since epoch); `outlier` repeats consumption only at flagged hours. */
export interface HourlyPoint {
  t: number
  consumption: number | null
  voltage: number | null
  current: number | null
  powerFactor: number | null
  outlier: number | null
}

export interface DailyPoint {
  day: string // YYYY-MM-DD (UTC)
  label: string // "12 sep"
  consumption: number
  inChange: boolean
}

export interface TimeRange {
  from: number
  to: number
}

export interface EventMarker {
  t: number
  label: string
}

export function hourlySeries(readings: Reading[], outlierTimestamps: string[] = []): HourlyPoint[] {
  const outliers = new Set(outlierTimestamps.map((ts) => Date.parse(ts)))
  return readings.map((r) => {
    const t = Date.parse(r.timestamp)
    return {
      t,
      consumption: r.consumption_kwh,
      voltage: r.voltage_v,
      current: r.current_a,
      powerFactor: r.power_factor,
      outlier: outliers.has(t) ? r.consumption_kwh : null,
    }
  })
}

/** Daily sums per UTC day; days on or after the change start's day are flagged. */
export function dailySeries(readings: Reading[], changeStart: string | null): DailyPoint[] {
  const changeDay = changeStart ? changeStart.slice(0, 10) : null
  const sums = new Map<string, number>()
  for (const r of readings) {
    const day = r.timestamp.slice(0, 10)
    sums.set(day, (sums.get(day) ?? 0) + (r.consumption_kwh ?? 0))
  }
  return [...sums.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([day, consumption]) => ({
      day,
      label: formatDataDay(`${day}T00:00:00Z`),
      consumption: Math.round(consumption * 10) / 10,
      inChange: changeDay !== null && day >= changeDay,
    }))
}

/** The daily baseline spread evenly over the day, for the hourly chart. */
export function hourlyBaseline(baselineKwh: number): number {
  return baselineKwh / 24
}

/** From the change start to the last reading, or null when there is no change or no reading after it. */
export function changeRange(changeStart: string | null, points: { t: number }[]): TimeRange | null {
  if (!changeStart || points.length === 0) return null
  const from = Date.parse(changeStart)
  const to = points[points.length - 1].t
  return from <= to ? { from, to } : null
}

const DAY_MS = 24 * 60 * 60 * 1000

/** UTC midnights inside the plotted range, every `step` days, for day-labelled time axes. */
export function dayTicks(points: { t: number }[], step = 1): number[] {
  if (points.length === 0) return []
  const first = points[0].t
  const last = points[points.length - 1].t
  const ticks: number[] = []
  for (let t = Math.ceil(first / DAY_MS) * DAY_MS; t <= last; t += step * DAY_MS) ticks.push(t)
  return ticks
}

/** Events inside the plotted range, as vertical markers. */
export function eventMarkers(events: MeterEvent[], points: { t: number }[]): EventMarker[] {
  if (points.length === 0) return []
  const first = points[0].t
  const last = points[points.length - 1].t
  return events
    .map((e) => ({ t: Date.parse(e.timestamp), label: e.type ?? 'Evento' }))
    .filter((m) => m.t >= first && m.t <= last)
}
