import { stageColor } from '@/lib/stages'
import { cn } from '@/lib/utils'

export function StageDot({ stage, className }: { stage: string; className?: string }) {
  return <span aria-hidden className={cn('size-2 shrink-0 rounded-full', className)} style={{ background: stageColor(stage) }} />
}

// Stage shows a status value as its coloured dot and name.
export function Stage({ stage }: { stage: string }) {
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5">
      <StageDot stage={stage} />
      <span className="truncate">{stage}</span>
    </span>
  )
}
