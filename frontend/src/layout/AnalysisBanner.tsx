import { Check, Sparkles, TriangleAlert, X } from 'lucide-react'
import { Link } from 'react-router'

import { Button } from '../components/Button'
import { useAnalysis } from '../features/analysis/analysisContext'
import { PIPELINE_STEPS } from '../lib/labels'

function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`
}

function CloseButton({ onClick }: { onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} aria-label="Cerrar" className="text-muted hover:text-text">
      <X className="size-4" aria-hidden />
    </button>
  )
}

/** Progress and outcome of the tracked analysis run, under the header on every screen. */
export function AnalysisBanner() {
  const { run, startError, start, dismiss } = useAnalysis()

  if (startError) {
    return (
      <div role="alert" className="mx-8 mt-4 flex items-center justify-between gap-4 rounded-xl border border-danger/40 bg-danger/10 px-5 py-3 text-sm">
        <span className="flex items-center gap-2">
          <TriangleAlert className="size-4 text-danger" aria-hidden />
          No se pudo iniciar el análisis: {startError.message}
        </span>
        <span className="flex items-center gap-3">
          <Button variant="outline" onClick={start}>Reintentar</Button>
          <CloseButton onClick={dismiss} />
        </span>
      </div>
    )
  }
  if (!run) return null

  if (run.status === 'COMPLETED') {
    const s = run.summary
    return (
      <div role="status" className="mx-8 mt-4 flex items-center justify-between gap-4 rounded-xl border border-primary/40 bg-primary/10 px-5 py-3 text-sm">
        <span className="flex items-center gap-2 font-medium">
          <Check className="size-4 text-primary" aria-hidden />
          {s
            ? `${plural(s.anomalies_detected, 'anomalía detectada', 'anomalías detectadas')} · ${plural(s.high_priority, 'requiere', 'requieren')} atención prioritaria`
            : 'Análisis completado'}
        </span>
        <span className="flex items-center gap-4">
          <Link to="/anomalies" className="text-primary hover:underline">Ver anomalías →</Link>
          <CloseButton onClick={dismiss} />
        </span>
      </div>
    )
  }

  if (run.status === 'FAILED') {
    return (
      <div role="alert" className="mx-8 mt-4 flex items-center justify-between gap-4 rounded-xl border border-danger/40 bg-danger/10 px-5 py-3 text-sm">
        <span className="flex items-center gap-2">
          <TriangleAlert className="size-4 text-danger" aria-hidden />
          El análisis falló: {run.error ?? 'error desconocido'}
        </span>
        <span className="flex items-center gap-3">
          <Button variant="outline" onClick={start}>Reintentar</Button>
          <CloseButton onClick={dismiss} />
        </span>
      </div>
    )
  }

  const found = PIPELINE_STEPS.findIndex((s) => s.key === run.current_step)
  const current = found === -1 ? 0 : found
  return (
    <div role="status" aria-label="Análisis IA en curso" className="mx-8 mt-4 rounded-xl border border-border bg-surface px-5 py-4 text-sm">
      <div className="flex items-center justify-between">
        <span className="flex items-center gap-2 font-medium">
          <Sparkles className="size-4 text-accent" aria-hidden />
          Análisis IA en curso
        </span>
        <span className="text-xs text-muted">
          Paso {current + 1} de {PIPELINE_STEPS.length}
        </span>
      </div>
      <ol className="mt-3 flex flex-wrap items-center gap-2">
        {PIPELINE_STEPS.map((step, i) => {
          const state = i < current ? 'done' : i === current ? 'current' : 'pending'
          const cls =
            state === 'done'
              ? 'border-primary/40 bg-primary/10 text-primary'
              : state === 'current'
                ? 'border-accent bg-accent/15 text-text'
                : 'border-border text-muted'
          return (
            <li key={step.key} data-state={state} className="flex items-center gap-2">
              {i > 0 && <span className="text-muted" aria-hidden>→</span>}
              <span className={`flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs ${cls}`}>
                {state === 'done' && <Check className="size-3" aria-hidden />}
                {step.label}
              </span>
            </li>
          )
        })}
      </ol>
      <div className="mt-3 h-1 rounded bg-surface-2">
        <div className="h-1 rounded bg-linear-to-r from-primary to-accent transition-all" style={{ width: `${((current + 1) / PIPELINE_STEPS.length) * 100}%` }} />
      </div>
    </div>
  )
}
