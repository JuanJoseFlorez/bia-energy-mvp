import { describe, expect, it } from 'vitest'

import {
  formatCompactKwh,
  formatConfidence,
  formatDataDateTime,
  formatDataDay,
  formatKwh,
  formatLocalDateTime,
  formatNumber,
  formatPeriod,
  formatSignedPercent,
} from './format'

describe('numbers', () => {
  it.each([
    [2207.6, 1, '2.207,6'],
    [1052.7, 1, '1.052,7'],
    [155250.85, 2, '155.250,85'],
    [0.5, 0, '1'],
    [-0.04, 1, '0,0'],
  ])('formatNumber(%s, %s) = %s', (value, decimals, want) => {
    expect(formatNumber(value, decimals)).toBe(want)
  })

  it('formats kWh with one decimal', () => {
    expect(formatKwh(2207.6)).toBe('2.207,6 kWh')
  })

  it.each([
    [155250.85, '155,3 K kWh'],
    [2_500_000, '2,5 M kWh'],
    [820, '820,0 kWh'],
  ])('formatCompactKwh(%s) = %s', (value, want) => {
    expect(formatCompactKwh(value)).toBe(want)
  })

  it.each([
    [109.7, '+109,7 %'],
    [-1.4, '−1,4 %'],
    [0, '0,0 %'],
    [-0.04, '0,0 %'],
    [0.04, '0,0 %'],
    [1234.56, '+1.234,6 %'],
  ])('formatSignedPercent(%s) = %s', (value, want) => {
    expect(formatSignedPercent(value)).toBe(want)
  })

  it.each([
    [0.96, '96 %'],
    [0.915, '92 %'],
    [1, '100 %'],
  ])('formatConfidence(%s) = %s', (value, want) => {
    expect(formatConfidence(value)).toBe(want)
  })
})

describe('dates', () => {
  it('formats dataset timestamps in UTC', () => {
    expect(formatDataDateTime('2026-09-12T14:00:00Z')).toBe('12 sep 14:00')
    expect(formatDataDay('2026-09-01T05:00:00Z')).toBe('1 sep')
  })

  it('formats the reading period', () => {
    expect(formatPeriod('2026-09-01T00:00:00Z', '2026-09-14T23:00:00Z')).toBe('1 sep – 14 sep 2026')
  })

  it('formats run timestamps in local time (UTC in tests)', () => {
    expect(formatLocalDateTime('2026-09-27T20:04:00Z')).toBe('27 sep 20:04')
  })
})
