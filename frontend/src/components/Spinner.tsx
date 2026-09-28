import { LoaderCircle } from 'lucide-react'

export function Spinner({ label = 'Cargando…' }: { label?: string }) {
  return (
    <div role="status" className="flex items-center justify-center gap-2 py-10 text-sm text-muted">
      <LoaderCircle className="size-4 animate-spin" aria-hidden />
      {label}
    </div>
  )
}
