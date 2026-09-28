import type { ButtonHTMLAttributes } from 'react'

type Variant = 'primary' | 'outline' | 'ghost'

const VARIANTS: Record<Variant, string> = {
  primary: 'bg-primary text-bg hover:bg-primary/90 font-semibold',
  outline: 'border border-border text-text hover:bg-surface-2',
  ghost: 'text-muted hover:bg-surface-2 hover:text-text',
}

export function Button({ variant = 'primary', className = '', type = 'button', ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }) {
  return (
    <button
      type={type}
      className={`inline-flex items-center justify-center gap-2 rounded-lg px-4 py-2 text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${VARIANTS[variant]} ${className}`}
      {...props}
    />
  )
}
