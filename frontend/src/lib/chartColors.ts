// Recharts draws SVG attributes, which cannot read CSS variables reliably: these mirror the
// @theme colors in index.css — keep both in sync.
export const CHART_COLORS = {
  primary: '#14d9b5',
  primaryStrong: '#0e9f86',
  accent: '#6d5dfc',
  danger: '#ff6b6b',
  warning: '#f5a524',
  info: '#8ab4ff',
  grid: '#24262e',
  axis: '#8a8f9c',
  surface: '#0f1015',
} as const
