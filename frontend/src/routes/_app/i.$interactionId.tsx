import { createFileRoute, useRouter } from '@tanstack/react-router'
import { InteractionDetails } from '@/components/interaction'

export const Route = createFileRoute('/_app/i/$interactionId')({ component: InteractionPage })

function InteractionPage() {
  const { interactionId } = Route.useParams()
  const router = useRouter()
  return <InteractionDetails interactionId={interactionId} onClose={() => router.history.back()} />
}
