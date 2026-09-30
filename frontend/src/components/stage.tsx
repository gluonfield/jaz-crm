import { stageColor } from '@/lib/stages'
import type { Attribute } from '@/lib/types'
import { cn } from '@/lib/utils'

export function StageDot({ attribute, stage, className }: { attribute: Attribute; stage: string; className?: string }) {
  return <span aria-hidden className={cn('size-2 shrink-0 rounded-full', className)} style={{ background: stageColor(attribute, stage) }} />
}

// Stage shows a status value as its coloured dot and name.
export function Stage({ attribute, stage }: { attribute: Attribute; stage: string }) {
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5">
      <StageDot attribute={attribute} stage={stage} />
      <span className="truncate">{stage}</span>
    </span>
  )
}
