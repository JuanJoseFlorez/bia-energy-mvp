import { CartesianGrid, Line, LineChart, ReferenceArea, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

import type { Reading } from '../../api/types'
import { Card } from '../../components/Card'
import { CHART_COLORS } from '../../lib/chartColors'
import { formatDataDateTime, formatDataDay, formatNumber } from '../../lib/format'
import { changeRange, dayTicks, hourlySeries, type HourlyPoint } from './chartData'

const SERIES: { key: keyof Pick<HourlyPoint, 'voltage' | 'current' | 'powerFactor'>; title: string; unit: string; decimals: number }[] = [
  { key: 'voltage', title: 'Voltaje', unit: 'V', decimals: 1 },
  { key: 'current', title: 'Corriente', unit: 'A', decimals: 1 },
  { key: 'powerFactor', title: 'Factor de potencia', unit: '', decimals: 2 },
]

const axisProps = { stroke: CHART_COLORS.axis, fontSize: 11, tickLine: false, axisLine: false } as const

/** Voltage, current and power factor on the same time axis and change window (brief §7, §8). */
export function ElectricalCharts({ readings, changeStart }: { readings: Reading[]; changeStart: string | null }) {
  const points = hourlySeries(readings)
  const change = changeRange(changeStart, points)
  const ticks = dayTicks(points, 3)
  return (
    <div className="grid gap-4 lg:grid-cols-3">
      {SERIES.map((s) => (
        <Card key={s.key} title={s.unit ? `${s.title} (${s.unit})` : s.title}>
          <ResponsiveContainer width="100%" height={150}>
            <LineChart data={points} syncId="electrical" margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
              <CartesianGrid stroke={CHART_COLORS.grid} strokeDasharray="3 3" vertical={false} />
              <XAxis dataKey="t" type="number" scale="time" domain={['dataMin', 'dataMax']} ticks={ticks} tickFormatter={(t: number) => formatDataDay(new Date(t).toISOString())} {...axisProps} />
              <YAxis domain={['auto', 'auto']} width={44} tickFormatter={(v: number) => formatNumber(v, s.decimals === 2 ? 2 : 0)} {...axisProps} />
              <Tooltip
                contentStyle={{ backgroundColor: CHART_COLORS.surface, border: `1px solid ${CHART_COLORS.grid}`, borderRadius: 8, fontSize: 12 }}
                labelFormatter={(t) => formatDataDateTime(new Date(Number(t)).toISOString())}
                formatter={(v) => [`${formatNumber(Number(v), s.decimals)} ${s.unit}`.trim(), s.title]}
              />
              {change && <ReferenceArea x1={change.from} x2={change.to} fill={CHART_COLORS.danger} fillOpacity={0.08} stroke="none" />}
              <Line dataKey={s.key} stroke={CHART_COLORS.info} strokeWidth={1.2} dot={false} isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        </Card>
      ))}
    </div>
  )
}
