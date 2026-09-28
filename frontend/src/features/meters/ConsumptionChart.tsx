import { Bar, BarChart, CartesianGrid, Cell, Line, LineChart, ReferenceArea, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

import type { MeterEvent, Reading } from '../../api/types'
import { CHART_COLORS } from '../../lib/chartColors'
import { formatDataDateTime, formatDataDay, formatNumber } from '../../lib/format'
import { changeRange, dailySeries, dayTicks, eventMarkers, hourlyBaseline, hourlySeries } from './chartData'

export type ChartMode = 'hourly' | 'daily'

interface Props {
  readings: Reading[]
  mode: ChartMode
  baselineKwh: number | null
  changeStart: string | null
  outlierTimestamps?: string[]
  events?: MeterEvent[]
  height?: number
}

const axisProps = { stroke: CHART_COLORS.axis, fontSize: 12, tickLine: false, axisLine: false } as const
const tooltipStyle = { backgroundColor: CHART_COLORS.surface, border: `1px solid ${CHART_COLORS.grid}`, borderRadius: 8, fontSize: 12 }

/** Consumption against the baseline: hourly line (change window, outliers, events) or daily bars. */
export function ConsumptionChart({ readings, mode, baselineKwh, changeStart, outlierTimestamps = [], events = [], height = 300 }: Props) {
  if (mode === 'daily') {
    const days = dailySeries(readings, changeStart)
    return (
      <ResponsiveContainer width="100%" height={height}>
        <BarChart data={days} margin={{ top: 16, right: 8, left: 0, bottom: 0 }}>
          <CartesianGrid stroke={CHART_COLORS.grid} strokeDasharray="3 3" vertical={false} />
          <XAxis dataKey="label" {...axisProps} />
          <YAxis {...axisProps} width={56} tickFormatter={(v: number) => formatNumber(v, 0)} />
          <Tooltip contentStyle={tooltipStyle} formatter={(v) => [`${formatNumber(Number(v))} kWh`, 'Consumo']} cursor={{ fill: CHART_COLORS.grid }} />
          <Bar dataKey="consumption" radius={[4, 4, 0, 0]} isAnimationActive={false}>
            {days.map((d) => (
              <Cell key={d.day} fill={d.inChange ? CHART_COLORS.danger : CHART_COLORS.primaryStrong} />
            ))}
          </Bar>
          {baselineKwh !== null && (
            <ReferenceLine y={baselineKwh} stroke={CHART_COLORS.accent} strokeDasharray="6 4" strokeWidth={2} label={{ value: 'Baseline', fill: CHART_COLORS.accent, fontSize: 12, position: 'insideTopRight' }} />
          )}
        </BarChart>
      </ResponsiveContainer>
    )
  }

  const points = hourlySeries(readings, outlierTimestamps)
  const change = changeRange(changeStart, points)
  const markers = eventMarkers(events, points)
  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={points} margin={{ top: 16, right: 8, left: 0, bottom: 0 }}>
        <CartesianGrid stroke={CHART_COLORS.grid} strokeDasharray="3 3" vertical={false} />
        <XAxis dataKey="t" type="number" scale="time" domain={['dataMin', 'dataMax']} ticks={dayTicks(points)} interval="preserveStartEnd" tickFormatter={(t: number) => formatDataDay(new Date(t).toISOString())} {...axisProps} />
        <YAxis {...axisProps} width={56} tickFormatter={(v: number) => formatNumber(v, 0)} />
        <Tooltip contentStyle={tooltipStyle} labelFormatter={(t) => formatDataDateTime(new Date(Number(t)).toISOString())} formatter={(v, name) => [`${formatNumber(Number(v))} kWh`, name === 'outlier' ? 'Fuera de lo esperado' : 'Consumo']} />
        {change && <ReferenceArea x1={change.from} x2={change.to} fill={CHART_COLORS.danger} fillOpacity={0.08} stroke="none" />}
        {baselineKwh !== null && (
          <ReferenceLine y={hourlyBaseline(baselineKwh)} stroke={CHART_COLORS.accent} strokeDasharray="6 4" strokeWidth={2} label={{ value: 'Baseline', fill: CHART_COLORS.accent, fontSize: 12, position: 'insideTopRight' }} />
        )}
        {markers.map((m) => (
          <ReferenceLine key={m.t} x={m.t} stroke={CHART_COLORS.warning} label={{ value: m.label, fill: CHART_COLORS.warning, fontSize: 11, position: 'insideTopLeft' }} />
        ))}
        <Line dataKey="consumption" stroke={CHART_COLORS.primary} strokeWidth={1.5} dot={false} isAnimationActive={false} />
        <Line dataKey="outlier" stroke="none" dot={{ r: 4, fill: CHART_COLORS.danger, strokeWidth: 0 }} activeDot={false} isAnimationActive={false} legendType="none" />
      </LineChart>
    </ResponsiveContainer>
  )
}
