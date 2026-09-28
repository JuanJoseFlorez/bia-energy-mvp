import { LoaderCircle, Sparkles } from 'lucide-react'

import { Button } from '../components/Button'
import { useAnalysis } from '../features/analysis/analysisContext'

export function RunAnalysisButton() {
  const { isActive, start } = useAnalysis()
  return (
    <Button onClick={start} disabled={isActive}>
      {isActive ? <LoaderCircle className="size-4 animate-spin" aria-hidden /> : <Sparkles className="size-4" aria-hidden />}
      {isActive ? 'Analizando…' : 'Run AI Analysis'}
    </Button>
  )
}
