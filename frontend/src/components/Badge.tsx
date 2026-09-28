import type { ReactNode } from 'react'

import type { Tone } from '../lib/labels'
import { TONE_BADGE } from './tone'

export function Badge({ tone, children }: { tone: Tone; children: ReactNode }) {
  return (
    <span className={`inline-flex items-center gap-1 whitespace-nowrap rounded-md border px-2 py-0.5 text-xs font-medium ${TONE_BADGE[tone]}`}>
      {children}
    </span>
  )
}
