import type { Tone } from '../lib/labels'

/** Text + border + tinted background per tone, for badges and pills. */
export const TONE_BADGE: Record<Tone, string> = {
  primary: 'text-primary border-primary/40 bg-primary/10',
  accent: 'text-accent border-accent/50 bg-accent/15',
  danger: 'text-danger border-danger/40 bg-danger/10',
  warning: 'text-warning border-warning/40 bg-warning/10',
  info: 'text-info border-info/40 bg-info/10',
  neutral: 'text-neutral border-border bg-surface-2',
}

/** Text color per tone. */
export const TONE_TEXT: Record<Tone, string> = {
  primary: 'text-primary',
  accent: 'text-accent',
  danger: 'text-danger',
  warning: 'text-warning',
  info: 'text-info',
  neutral: 'text-neutral',
}

/** Solid fill per tone (bars). */
export const TONE_FILL: Record<Tone, string> = {
  primary: 'bg-primary',
  accent: 'bg-accent',
  danger: 'bg-danger',
  warning: 'bg-warning',
  info: 'bg-info',
  neutral: 'bg-neutral',
}
