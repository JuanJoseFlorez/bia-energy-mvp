import { describe, expect, it } from 'vitest'

import type { MeterEvent } from '../../api/types'
import { readings } from '../../test/screenFixtures'
import { changeRange, dailySeries, dayTicks, eventMarkers, hourlyBaseline, hourlySeries } from './chartData'

describe('chart data', () => {
  it('builds hourly points on a numeric time axis and flags outliers', () => {
    const points = hourlySeries(readings.items, ['2026-09-12T14:00:00Z'])

    expect(points.map((p) => p.t)).toEqual(readings.items.map((r) => Date.parse(r.timestamp)))
    expect(points.map((p) => p.outlier)).toEqual([null, null, 92, null])
    expect(points[0]).toMatchObject({ consumption: 44, voltage: 220, current: 150, powerFactor: 0.86 })
  })

  it('sums consumption per UTC day and flags days from the change start', () => {
    expect(dailySeries(readings.items, '2026-09-12T14:00:00Z')).toEqual([
      { day: '2026-09-12', label: '12 sep', consumption: 181, inChange: true },
      { day: '2026-09-13', label: '13 sep', consumption: 95, inChange: true },
    ])
    expect(dailySeries(readings.items, null).every((d) => !d.inChange)).toBe(true)
    expect(dailySeries([], null)).toEqual([])
  })

  it('spreads the daily baseline over 24 hours', () => {
    expect(hourlyBaseline(1052.7)).toBeCloseTo(43.8625)
  })

  it('spans the change window from its start to the last reading', () => {
    const points = hourlySeries(readings.items)
    expect(changeRange('2026-09-12T14:00:00Z', points)).toEqual({ from: Date.parse('2026-09-12T14:00:00Z'), to: Date.parse('2026-09-13T00:00:00Z') })
    expect(changeRange(null, points)).toBeNull()
    expect(changeRange('2026-09-20T00:00:00Z', points)).toBeNull()
    expect(changeRange('2026-09-12T14:00:00Z', [])).toBeNull()
  })

  it('puts day ticks at UTC midnights inside the range', () => {
    const points = hourlySeries(readings.items)
    expect(dayTicks(points)).toEqual([Date.parse('2026-09-13T00:00:00Z')])
    const span = [{ t: Date.parse('2026-09-01T00:00:00Z') }, { t: Date.parse('2026-09-07T23:00:00Z') }]
    expect(dayTicks(span, 3)).toEqual(['2026-09-01', '2026-09-04', '2026-09-07'].map((d) => Date.parse(`${d}T00:00:00Z`)))
    expect(dayTicks([])).toEqual([])
  })

  it('keeps only events inside the plotted range', () => {
    const events: MeterEvent[] = [
      { id: 3, meter_id: 'M-109', timestamp: '2026-09-12T14:00:00Z', type: 'UNKNOWN', description: null },
      { id: 9, meter_id: 'M-109', timestamp: '2026-09-01T00:00:00Z', type: 'OTHER', description: null },
      { id: 10, meter_id: 'M-109', timestamp: '2026-09-12T13:00:00Z', type: null, description: null },
    ]
    expect(eventMarkers(events, hourlySeries(readings.items))).toEqual([
      { t: Date.parse('2026-09-12T14:00:00Z'), label: 'UNKNOWN' },
      { t: Date.parse('2026-09-12T13:00:00Z'), label: 'Evento' },
    ])
  })
})
