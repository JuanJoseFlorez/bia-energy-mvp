// Mirrors the backend JSON (brief §16 entities plus analysis-derived fields). Timestamps are RFC3339 UTC strings.

export type AnomalyType = 'REAL_ANOMALY' | 'EXPLAINABLE_ANOMALY' | 'FALSE_POSITIVE' | 'DATA_QUALITY'
export type Severity = 'LOW' | 'MEDIUM' | 'HIGH'
export type Health = 'OK' | 'ALERT' | 'CRITICAL'
export type AnomalyStatus = 'PENDING' | 'INVESTIGATING' | 'VALIDATED' | 'DISMISSED' | 'RESOLVED'
export type RunStatus = 'PENDING' | 'RUNNING' | 'COMPLETED' | 'FAILED'
export type ExplanationSource = 'llm' | 'template'

export interface ListResult<T> {
  items: T[]
  total: number
}

export interface Metrics {
  current_kwh: number
  baseline_kwh: number
  variation_pct: number
  change_start: string | null
}

export interface AnomalySummary {
  id: number
  meter_id: string
  type: AnomalyType | null
  severity: Severity | null
  confidence: number | null
  priority: number | null
}

export interface Meter {
  id: number
  meter_id: string
  name: string | null
  location: string | null
  status: string | null
  created_at: string | null
  period_consumption_kwh: number
  health: Health | null
  metrics: Metrics | null
  anomaly: AnomalySummary | null
}

export interface MeterAnomaly extends AnomalySummary {
  reason: string | null
  recommended_action: string | null
  status: AnomalyStatus | null
  detected_at: string | null
}

export interface MeterEvent {
  id: number
  meter_id: string
  timestamp: string
  type: string | null
  description: string | null
}

export interface MeterDetail extends Omit<Meter, 'anomaly'> {
  anomaly: MeterAnomaly | null
  events: MeterEvent[]
}

export interface Reading {
  id: number
  meter_id: string
  timestamp: string
  consumption_kwh: number | null
  voltage_v: number | null
  current_a: number | null
  power_factor: number | null
  status: string | null
}

export interface ReadingsResult extends ListResult<Reading> {
  meter_id: string
}

export interface DashboardSummary {
  meters_count: number
  period: { from: string; to: string } | null
  total_consumption_kwh: number
  last_analysis: { id: number; status: RunStatus; started_at: string | null; finished_at: string | null } | null
  anomalies: {
    analysis_id: number
    analysis_finished_at: string
    detected: number
    high_priority: number
    avg_confidence: number | null
    by_type: Record<AnomalyType, number>
  } | null
}

export interface AnalysisRun {
  id: number
  status: RunStatus
  current_step: string | null
  started_at: string | null
  updated_at: string
  finished_at: string | null
  summary: { anomalies_detected: number; high_priority: number } | null
  error: string | null
}

export interface AnomalyItem {
  id: number
  analysis_id: number
  meter_id: string
  detected_at: string | null
  anomaly: boolean
  type: AnomalyType | null
  severity: Severity | null
  confidence: number | null
  priority: number | null
  reason: string | null
  recommended_action: string | null
  explanation_source: ExplanationSource | null
  status: AnomalyStatus
}

export interface AnomalyList extends ListResult<AnomalyItem> {
  analysis_id: number | null
}

/** Evidence as computed by engine-ai (stored verbatim by the backend). */
export interface Evidence {
  baseline_kwh: number
  current_kwh: number
  variation_pct: number
  change_start: string | null
  baseline_reliable: boolean
  shift_pct: number | null
  effect_size: number | null
  changed_vars: Record<string, { before: number; after: number }> | null
  transient: { start: string; end: string; hours: number; mean_deviation_pct: number; effect_size: number } | null
  failed_checks: { check: string; hours: number }[]
  outlier_timestamps: string[]
  profile_correlation: number | null
  related_event_ids: number[]
  signals: string[]
}

export interface AnomalyDetail extends AnomalyItem {
  explanation: string | null
  evidence: Evidence | null
  related_events: MeterEvent[]
  readings_window: { from: string; to: string } | null
  next_statuses: AnomalyStatus[]
}

export interface Session {
  token: string
  user: { username: string }
}
