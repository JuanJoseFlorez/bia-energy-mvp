import type { Evidence } from '../../api/types'
import { TABLE, TD, TH, THEAD_ROW } from '../../components/table'
import { formatDataDateTime, formatNumber, formatSignedPercent } from '../../lib/format'
import { variableLabel } from '../../lib/labels'

/** Electrical variables before and after the change (evidence.changed_vars), plus a transient summary. */
export function ChangedVarsTable({ evidence }: { evidence: Evidence }) {
  const vars = Object.entries(evidence.changed_vars ?? {})
  const t = evidence.transient
  if (vars.length === 0 && !t) return <p className="text-sm text-muted">Sin un cambio sostenido antes y después; revisa el comportamiento de las variables eléctricas abajo.</p>
  return (
    <div className="flex flex-col gap-3">
      {vars.length > 0 && (
        <table className={TABLE}>
          <thead>
            <tr className={THEAD_ROW}>
              <th className={TH}>Variable</th>
              <th className={`${TH} text-right`}>Antes</th>
              <th className={`${TH} text-right`}>Después</th>
              <th className={`${TH} text-right`}>Δ</th>
            </tr>
          </thead>
          <tbody>
            {vars.map(([name, v]) => {
              const decimals = name === 'power_factor' ? 2 : 1
              const delta = v.before === 0 ? null : ((v.after - v.before) / Math.abs(v.before)) * 100
              return (
                <tr key={name} className="border-b border-border last:border-0">
                  <td className={TD}>{variableLabel(name)}</td>
                  <td className={`${TD} text-right`}>{formatNumber(v.before, decimals)}</td>
                  <td className={`${TD} text-right`}>{formatNumber(v.after, decimals)}</td>
                  <td className={`${TD} text-right ${delta !== null && Math.abs(delta) >= 5 ? 'text-danger' : 'text-muted'}`}>{delta === null ? '—' : formatSignedPercent(delta)}</td>
                </tr>
              )
            })}
          </tbody>
        </table>
      )}
      {t && (
        <p className="text-sm">
          Cambio transitorio: {formatDataDateTime(t.start)} → {formatDataDateTime(t.end)} ({t.hours} h, {formatSignedPercent(t.mean_deviation_pct)} frente al baseline)
        </p>
      )}
    </div>
  )
}
