import type { AnomalyStatus, AnomalyType, ExplanationSource, Health, RunStatus, Severity } from '../api/types'

/** Color family of a badge or value; each maps to a theme color. */
export type Tone = 'primary' | 'accent' | 'danger' | 'warning' | 'info' | 'neutral'

export interface Meta {
  label: string
  tone: Tone
}

const NONE: Meta = { label: '—', tone: 'neutral' }

const TYPES: Record<AnomalyType, Meta & { action: string }> = {
  REAL_ANOMALY: { label: 'Anomalía real', tone: 'danger', action: 'Investigar' },
  DATA_QUALITY: { label: 'Calidad de datos', tone: 'warning', action: 'Validar' },
  EXPLAINABLE_ANOMALY: { label: 'Anomalía explicable', tone: 'info', action: 'Validar operación' },
  FALSE_POSITIVE: { label: 'Falso positivo', tone: 'neutral', action: 'No escalar' },
}

const SEVERITIES: Record<Severity, Meta> = {
  HIGH: { label: 'Alta', tone: 'danger' },
  MEDIUM: { label: 'Media', tone: 'warning' },
  LOW: { label: 'Baja', tone: 'neutral' },
}

const HEALTH: Record<Health, Meta> = {
  CRITICAL: { label: 'Crítico', tone: 'danger' },
  ALERT: { label: 'Alerta', tone: 'warning' },
  OK: { label: 'Normal', tone: 'primary' },
}

const ANOMALY_STATUSES: Record<AnomalyStatus, Meta> = {
  PENDING: { label: 'Pendiente', tone: 'warning' },
  INVESTIGATING: { label: 'En investigación', tone: 'info' },
  VALIDATED: { label: 'Validada', tone: 'primary' },
  DISMISSED: { label: 'Descartada', tone: 'neutral' },
  RESOLVED: { label: 'Resuelta', tone: 'primary' },
}

const RUN_STATUSES: Record<RunStatus, Meta> = {
  PENDING: { label: 'En cola', tone: 'neutral' },
  RUNNING: { label: 'En curso', tone: 'accent' },
  COMPLETED: { label: 'Completado', tone: 'primary' },
  FAILED: { label: 'Fallido', tone: 'danger' },
}

const SOURCES: Record<ExplanationSource, Meta> = {
  llm: { label: 'IA (LLM)', tone: 'accent' },
  template: { label: 'Plantilla', tone: 'neutral' },
}

/** Button label for moving an anomaly to each workflow status. */
const TRANSITIONS: Record<AnomalyStatus, string> = {
  PENDING: 'Marcar pendiente',
  INVESTIGATING: 'Marcar en investigación',
  VALIDATED: 'Validar',
  RESOLVED: 'Resolver',
  DISMISSED: 'Descartar',
}

/** Steps of the analysis pipeline (brief §13), in order. */
export const PIPELINE_STEPS = [
  { key: 'READINGS', label: 'Lecturas' },
  { key: 'BASELINE', label: 'Baseline' },
  { key: 'DETECTION', label: 'Detección' },
  { key: 'CORRELATION', label: 'Correlación' },
  { key: 'EVENTS', label: 'Eventos' },
  { key: 'EXPLANATION', label: 'Explicación' },
  { key: 'RECOMMENDATION', label: 'Recomendación' },
] as const

/** Signals and data-quality checks produced by engine-ai. */
const EVIDENCE: Record<string, string> = {
  no_explaining_event: 'Sin evento operativo que lo explique',
  explained_by_event: 'Explicado por un evento operativo',
  data_quality_event: 'Evento de calidad de datos registrado',
  power_factor_drop: 'Caída del factor de potencia',
  power_ratio_shift: 'Cambio en la relación potencia/consumo',
  current_follows: 'La corriente acompaña el cambio',
  voltage_out_of_range: 'Voltaje fuera de rango',
  voltage_jump: 'Saltos bruscos de voltaje',
  current_jump_flat_consumption: 'Saltos de corriente con consumo estable',
  power_ratio_inconsistent: 'Potencia (V·I·FP) inconsistente con el consumo',
  power_factor_jump: 'Saltos bruscos del factor de potencia',
  power_factor_out_of_range: 'Factor de potencia fuera de rango',
  flatline: 'Lecturas repetidas (flatline)',
}

const VARIABLES: Record<string, string> = {
  consumption_kwh: 'Consumo (kWh)',
  voltage_v: 'Voltaje (V)',
  current_a: 'Corriente (A)',
  power_factor: 'Factor de potencia',
}

export function anomalyTypeMeta(type: AnomalyType | null): Meta & { action: string } {
  return type ? TYPES[type] : { ...NONE, action: 'Ver' }
}

export function severityMeta(severity: Severity | null): Meta {
  return severity ? SEVERITIES[severity] : NONE
}

export function healthMeta(health: Health | null): Meta {
  return health ? HEALTH[health] : { label: 'Sin análisis', tone: 'neutral' }
}

export function anomalyStatusMeta(status: AnomalyStatus | null): Meta {
  return status ? ANOMALY_STATUSES[status] : NONE
}

export function runStatusMeta(status: RunStatus): Meta {
  return RUN_STATUSES[status]
}

export function sourceMeta(source: ExplanationSource | null): Meta {
  return source ? SOURCES[source] : NONE
}

export function transitionLabel(status: AnomalyStatus): string {
  return TRANSITIONS[status]
}

/** Confidence band (brief §11): ≥ 0.8 Alta, ≥ 0.6 Media, else Baja. */
export function confidenceLabel(confidence: number): string {
  if (confidence >= 0.8) return 'Alta'
  if (confidence >= 0.6) return 'Media'
  return 'Baja'
}

export function evidenceLabel(code: string): string {
  return EVIDENCE[code] ?? code
}

export function variableLabel(name: string): string {
  return VARIABLES[name] ?? name
}
