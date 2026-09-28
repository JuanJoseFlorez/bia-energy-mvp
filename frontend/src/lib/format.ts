// Display formatting for Spanish (Colombia): "2.207,6 kWh", "+109,7 %", "12 sep 14:00".

const MONTHS = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic']
const MINUS = '−'

export function formatNumber(value: number, decimals = 1): string {
  const rounded = roundTo(value, decimals)
  return new Intl.NumberFormat('es-CO', { minimumFractionDigits: decimals, maximumFractionDigits: decimals }).format(rounded)
}

export function formatKwh(value: number): string {
  return `${formatNumber(value)} kWh`
}

/** Large totals for KPI cards: 155.250,85 → "155,3 K kWh". */
export function formatCompactKwh(value: number): string {
  const abs = Math.abs(value)
  if (abs >= 1_000_000) return `${formatNumber(value / 1_000_000)} M kWh`
  if (abs >= 1_000) return `${formatNumber(value / 1_000)} K kWh`
  return formatKwh(value)
}

/** Signed percentage with one decimal; values that round to zero print "0,0 %" (never "-0,0"). */
export function formatSignedPercent(value: number): string {
  const rounded = roundTo(value, 1)
  if (rounded === 0) return `${formatNumber(0)} %`
  const sign = rounded > 0 ? '+' : MINUS
  return `${sign}${formatNumber(Math.abs(rounded))} %`
}

/** Confidence 0–1 as a whole percentage: 0.96 → "96 %". */
export function formatConfidence(value: number): string {
  return `${Math.round(value * 100)} %`
}

/** Dataset timestamps are UTC (same as the engine's texts): "12 sep 14:00". */
export function formatDataDateTime(iso: string): string {
  const d = new Date(iso)
  return `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`
}

/** Dataset day in UTC: "12 sep". */
export function formatDataDay(iso: string): string {
  const d = new Date(iso)
  return `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]}`
}

/** Reading period in UTC: "1 sep – 14 sep 2026". */
export function formatPeriod(from: string, to: string): string {
  return `${formatDataDay(from)} – ${formatDataDay(to)} ${new Date(to).getUTCFullYear()}`
}

/** Wall-clock moments (analysis runs) in the browser's time zone: "27 sep 20:04". */
export function formatLocalDateTime(iso: string): string {
  const d = new Date(iso)
  return `${d.getDate()} ${MONTHS[d.getMonth()]} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function roundTo(value: number, decimals: number): number {
  const factor = 10 ** decimals
  const r = Math.round(value * factor) / factor
  return Object.is(r, -0) ? 0 : r
}

function pad(n: number): string {
  return String(n).padStart(2, '0')
}
