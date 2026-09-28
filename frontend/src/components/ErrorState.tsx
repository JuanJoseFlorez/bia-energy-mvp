import { TriangleAlert } from 'lucide-react'

import { Button } from './Button'

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const message = error instanceof Error ? error.message : 'Error inesperado'
  return (
    <div role="alert" className="flex flex-col items-center gap-3 py-10 text-center">
      <TriangleAlert className="size-6 text-danger" aria-hidden />
      <p className="text-sm">No se pudieron cargar los datos: {message}</p>
      {onRetry && (
        <Button variant="outline" onClick={onRetry}>
          Reintentar
        </Button>
      )}
    </div>
  )
}
