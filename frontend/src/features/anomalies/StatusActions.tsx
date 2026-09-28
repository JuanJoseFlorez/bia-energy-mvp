import { useState } from 'react'

import { useUpdateAnomalyStatus } from '../../api/queries'
import type { AnomalyDetail, AnomalyStatus } from '../../api/types'
import { Button } from '../../components/Button'
import { ConfirmDialog } from '../../components/ConfirmDialog'
import { anomalyStatusMeta, transitionLabel } from '../../lib/labels'

/** Action workflow buttons from next_statuses (PATCH); dismissing asks for confirmation. */
export function StatusActions({ anomaly }: { anomaly: AnomalyDetail }) {
  const update = useUpdateAnomalyStatus(anomaly.id)
  const [confirming, setConfirming] = useState(false)
  const [done, setDone] = useState<AnomalyStatus | null>(null)

  function apply(status: AnomalyStatus) {
    setConfirming(false)
    update.mutate(status, { onSuccess: () => setDone(status) })
  }

  if (anomaly.next_statuses.length === 0) {
    return <p className="text-sm text-muted">Esta anomalía está {anomalyStatusMeta(anomaly.status).label.toLowerCase()}.</p>
  }
  return (
    <div className="flex flex-col gap-2">
      {anomaly.next_statuses.map((s, i) => (
        <Button key={s} variant={i === 0 ? 'primary' : 'outline'} disabled={update.isPending} onClick={() => (s === 'DISMISSED' ? setConfirming(true) : apply(s))}>
          {transitionLabel(s)}
        </Button>
      ))}
      {done && !update.isError && <p role="status" className="text-sm text-primary">Estado actualizado a {anomalyStatusMeta(done).label}</p>}
      {update.isError && (
        <p role="alert" className="text-sm text-danger">
          No se pudo actualizar: {update.error.message}
        </p>
      )}
      {confirming && (
        <ConfirmDialog
          title="Descartar anomalía"
          message="La anomalía dejará de requerir atención. Esta acción no se puede deshacer."
          confirmLabel="Descartar"
          onConfirm={() => apply('DISMISSED')}
          onCancel={() => setConfirming(false)}
        />
      )}
    </div>
  )
}
