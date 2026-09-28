import type { AnalysisRun, AnomalyItem, AnomalyList, DashboardSummary } from '../api/types'

export const emptyDashboard: DashboardSummary = {
  meters_count: 12,
  period: { from: '2026-09-01T00:00:00Z', to: '2026-09-14T23:00:00Z' },
  total_consumption_kwh: 155250.85,
  last_analysis: null,
  anomalies: null,
}

export const analyzedDashboard: DashboardSummary = {
  ...emptyDashboard,
  last_analysis: { id: 7, status: 'COMPLETED', started_at: '2026-09-27T20:04:00Z', finished_at: '2026-09-27T20:04:03Z' },
  anomalies: {
    analysis_id: 7,
    analysis_finished_at: '2026-09-27T20:04:03Z',
    detected: 4,
    high_priority: 2,
    avg_confidence: 0.915,
    by_type: { REAL_ANOMALY: 1, DATA_QUALITY: 1, EXPLAINABLE_ANOMALY: 1, FALSE_POSITIVE: 1 },
  },
}

function item(id: number, meter: string, type: AnomalyItem['type'], severity: AnomalyItem['severity'], priority: number): AnomalyItem {
  return {
    id,
    analysis_id: 7,
    meter_id: meter,
    detected_at: '2026-09-27T20:04:03Z',
    anomaly: type !== 'FALSE_POSITIVE',
    type,
    severity,
    confidence: 0.9,
    priority,
    reason: `reason ${meter}`,
    recommended_action: `action ${meter}`,
    explanation_source: 'template',
    status: 'PENDING',
  }
}

export const anomalyList: AnomalyList = {
  analysis_id: 7,
  total: 4,
  items: [
    item(31, 'M-109', 'REAL_ANOMALY', 'HIGH', 1),
    item(32, 'M-112', 'DATA_QUALITY', 'HIGH', 2),
    item(33, 'M-104', 'EXPLAINABLE_ANOMALY', 'MEDIUM', 3),
    item(34, 'M-106', 'FALSE_POSITIVE', 'LOW', 4),
  ],
}

export function runAt(status: AnalysisRun['status'], step: string | null, extra: Partial<AnalysisRun> = {}): AnalysisRun {
  return {
    id: 8,
    status,
    current_step: step,
    started_at: '2026-09-27T21:00:00Z',
    updated_at: '2026-09-27T21:00:01Z',
    finished_at: status === 'COMPLETED' || status === 'FAILED' ? '2026-09-27T21:00:03Z' : null,
    summary: status === 'COMPLETED' ? { anomalies_detected: 4, high_priority: 2 } : null,
    error: status === 'FAILED' ? 'analysis engine unavailable' : null,
    ...extra,
  }
}
