import { describe, expect, it } from 'vitest'

import {
  anomalyStatusMeta,
  anomalyTypeMeta,
  confidenceLabel,
  evidenceLabel,
  healthMeta,
  PIPELINE_STEPS,
  runStatusMeta,
  severityMeta,
  sourceMeta,
  transitionLabel,
  variableLabel,
} from './labels'

describe('labels', () => {
  it.each([
    ['REAL_ANOMALY', 'Anomalía real', 'danger', 'Investigar'],
    ['DATA_QUALITY', 'Calidad de datos', 'warning', 'Validar'],
    ['EXPLAINABLE_ANOMALY', 'Anomalía explicable', 'info', 'Validar operación'],
    ['FALSE_POSITIVE', 'Falso positivo', 'neutral', 'No escalar'],
  ] as const)('type %s → %s / %s / action %s', (type, label, tone, action) => {
    expect(anomalyTypeMeta(type)).toEqual({ label, tone, action })
  })

  it('severity, health and statuses', () => {
    expect(severityMeta('HIGH')).toEqual({ label: 'Alta', tone: 'danger' })
    expect(severityMeta('MEDIUM')).toEqual({ label: 'Media', tone: 'warning' })
    expect(severityMeta('LOW')).toEqual({ label: 'Baja', tone: 'neutral' })
    expect(healthMeta('CRITICAL').label).toBe('Crítico')
    expect(healthMeta('ALERT').label).toBe('Alerta')
    expect(healthMeta('OK')).toEqual({ label: 'Normal', tone: 'primary' })
    expect(healthMeta(null).label).toBe('Sin análisis')
    expect(anomalyStatusMeta('INVESTIGATING').label).toBe('En investigación')
    expect(runStatusMeta('FAILED')).toEqual({ label: 'Fallido', tone: 'danger' })
    expect(sourceMeta('llm').label).toBe('IA (LLM)')
    expect(transitionLabel('DISMISSED')).toBe('Descartar')
  })

  it('nulls render a dash', () => {
    expect(anomalyTypeMeta(null).label).toBe('—')
    expect(severityMeta(null).label).toBe('—')
    expect(sourceMeta(null).label).toBe('—')
  })

  it.each([
    [0.96, 'Alta'],
    [0.8, 'Alta'],
    [0.79, 'Media'],
    [0.6, 'Media'],
    [0.59, 'Baja'],
  ])('confidence %s → %s', (value, want) => {
    expect(confidenceLabel(value)).toBe(want)
  })

  it('pipeline has the seven §13 steps in order', () => {
    expect(PIPELINE_STEPS.map((s) => s.key)).toEqual([
      'READINGS', 'BASELINE', 'DETECTION', 'CORRELATION', 'EVENTS', 'EXPLANATION', 'RECOMMENDATION',
    ])
  })

  it('evidence and variable codes, with raw fallback', () => {
    expect(evidenceLabel('no_explaining_event')).toBe('Sin evento operativo que lo explique')
    expect(evidenceLabel('flatline')).toBe('Lecturas repetidas (flatline)')
    expect(evidenceLabel('brand_new_signal')).toBe('brand_new_signal')
    expect(variableLabel('current_a')).toBe('Corriente (A)')
    expect(variableLabel('other')).toBe('other')
  })
})
