import type { ReactNode } from 'react'

import type { Tone } from '../lib/labels'
import { TONE_TEXT } from './tone'

export function KpiCard({ label, value, tone, footer }: { label: string; value: ReactNode; tone?: Tone; footer?: ReactNode }) {
  return (
    <div className="flex flex-col gap-2 rounded-xl border border-border bg-surface px-5 py-4">
      <span className="text-sm text-muted">{label}</span>
      <span className={`text-2xl font-semibold tracking-tight ${tone ? TONE_TEXT[tone] : ''}`}>{value}</span>
      {footer && <div className="text-xs text-muted">{footer}</div>}
    </div>
  )
}
