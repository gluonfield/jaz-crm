import { type ReactNode, useState } from 'react'
import { InteractionDetails } from './interaction'
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from './ui/dialog'

export function InteractionDialog({ interactionId, children }: { interactionId: string; children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const close = () => setOpen(false)
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent
        aria-describedby={undefined}
        showCloseButton={false}
        className="flex max-h-[calc(100dvh-2rem)] w-[760px] max-w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden rounded-[var(--radius-card)] border-0 bg-raised p-0 text-ink duration-150 motion-reduce:animate-none sm:max-w-[760px]"
      >
        <DialogTitle className="sr-only">Conversation</DialogTitle>
        <InteractionDetails interactionId={interactionId} onClose={close} onNavigate={close} />
      </DialogContent>
    </Dialog>
  )
}
