import type { AnomalyDetail, ListResult, Meter, MeterDetail, Reading, ReadingsResult } from '../api/types'

export const meterList: ListResult<Meter> = {
  total: 2,
  items: [
    {
      id: 9, meter_id: 'M-109', name: 'Meter 109', location: 'Planta C', status: 'ACTIVE', created_at: '2026-09-26T22:00:00Z',
      period_consumption_kwh: 17526.04, health: 'CRITICAL',
      metrics: { current_kwh: 2207.6, baseline_kwh: 1052.7, variation_pct: 109.7, change_start: '2026-09-12T14:00:00Z' },
      anomaly: { id: 31, meter_id: 'M-109', type: 'REAL_ANOMALY', severity: 'HIGH', confidence: 0.99, priority: 1 },
    },
    {
      id: 1, meter_id: 'M-101', name: 'Meter 101', location: 'Planta A', status: 'ACTIVE', created_at: '2026-09-26T22:00:00Z',
      period_consumption_kwh: 10226.5, health: 'OK',
      metrics: { current_kwh: 820, baseline_kwh: 830, variation_pct: -1.2, change_start: null },
      anomaly: null,
    },
  ],
}

export const meterDetail: MeterDetail = {
  ...meterList.items[0],
  anomaly: { ...meterList.items[0].anomaly!, reason: 'Consumo +109,7 % sobre el baseline sin evento conocido.', recommended_action: 'Investigar medidor e instalación.', status: 'PENDING', detected_at: '2026-09-27T20:04:03Z' },
  events: [{ id: 3, meter_id: 'M-109', timestamp: '2026-09-12T14:00:00Z', type: 'UNKNOWN', description: 'No operational event reported' }],
}

function reading(i: number, ts: string, kwh: number): Reading {
  return { id: i, meter_id: 'M-109', timestamp: ts, consumption_kwh: kwh, voltage_v: 220, current_a: 150, power_factor: 0.86, status: 'OK' }
}

export const readings: ReadingsResult = {
  meter_id: 'M-109',
  total: 4,
  items: [
    reading(1, '2026-09-12T12:00:00Z', 44),
    reading(2, '2026-09-12T13:00:00Z', 45),
    reading(3, '2026-09-12T14:00:00Z', 92),
    reading(4, '2026-09-13T00:00:00Z', 95),
  ],
}

export const anomalyDetail: AnomalyDetail = {
  id: 31,
  analysis_id: 7,
  meter_id: 'M-109',
  detected_at: '2026-09-27T20:04:03Z',
  anomaly: true,
  type: 'REAL_ANOMALY',
  severity: 'HIGH',
  confidence: 0.99,
  priority: 1,
  reason: 'Consumo +109,7 % sobre el baseline sin evento conocido.',
  explanation: 'El consumo diario pasó de 1.052,7 kWh a 2.207,6 kWh desde el 12-sep 14:00.',
  recommended_action: 'Investigar medidor e instalación. Revisar cargas nuevas.',
  explanation_source: 'llm',
  status: 'PENDING',
  evidence: {
    baseline_kwh: 1052.7,
    current_kwh: 2207.6,
    variation_pct: 109.7,
    change_start: '2026-09-12T14:00:00Z',
    baseline_reliable: true,
    shift_pct: 112.4,
    effect_size: 18.4,
    changed_vars: { current_a: { before: 158.2, after: 317.9 }, power_factor: { before: 0.86, after: 0.72 } },
    transient: null,
    failed_checks: [],
    outlier_timestamps: ['2026-09-12T14:00:00Z'],
    profile_correlation: 0.94,
    related_event_ids: [3],
    signals: ['no_explaining_event', 'power_factor_drop', 'current_follows'],
  },
  related_events: [{ id: 3, meter_id: 'M-109', timestamp: '2026-09-12T14:00:00Z', type: 'UNKNOWN', description: 'No operational event reported' }],
  readings_window: { from: '2026-09-05T14:00:00Z', to: '2026-09-14T23:00:00Z' },
  next_statuses: ['INVESTIGATING', 'VALIDATED', 'DISMISSED'],
}
