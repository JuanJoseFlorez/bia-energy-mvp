import { Card } from '../components/Card'
import { EmptyState } from '../components/EmptyState'

/** Temporary page for sections delivered in the next frontend iteration. */
export function PlaceholderPage({ title }: { title: string }) {
  return (
    <Card>
      <EmptyState title={title} description="Esta sección estará disponible pronto." />
    </Card>
  )
}
