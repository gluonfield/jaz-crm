import { Button } from '@jaz/ui/button'
import { useRouter } from '@tanstack/react-router'
import { PanelLeft } from 'lucide-react'
import { Dialog as DialogPrimitive } from 'radix-ui'
import { type ReactNode, useEffect, useState } from 'react'
import { Dialog, DialogOverlay, DialogPortal, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Sidebar } from './sidebar'

// NavDrawer shows the sidebar as a drawer below md. It wraps the whole shell
// so each page header's NavButton is its trigger, and closes on navigation.
export function NavDrawer({ children }: { children: ReactNode }) {
  const router = useRouter()
  const [open, setOpen] = useState(false)
  useEffect(() => router.subscribe('onBeforeNavigate', () => setOpen(false)), [router])
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {children}
      <DialogPortal>
        <DialogOverlay className="motion-reduce:duration-0" />
        <DialogPrimitive.Content
          aria-describedby={undefined}
          // Focus returns to the menu button unless a layer opened from the drawer, such as search, has taken it.
          onCloseAutoFocus={(e) => document.activeElement !== document.body && e.preventDefault()}
          className="fixed inset-y-0 left-0 z-50 flex w-[232px] max-w-[85vw] flex-col overflow-y-auto bg-panel pb-[var(--safe-area-bottom)] shadow-[var(--shadow-raised)] outline-none duration-150 ease-out data-[state=closed]:animate-out data-[state=closed]:slide-out-to-left data-[state=open]:animate-in data-[state=open]:slide-in-from-left motion-reduce:duration-0"
        >
          <DialogTitle className="sr-only">Navigation</DialogTitle>
          <Sidebar className="w-full" />
        </DialogPrimitive.Content>
      </DialogPortal>
    </Dialog>
  )
}

export function NavButton() {
  return (
    <DialogTrigger asChild>
      <Button variant="ghost" size="icon" aria-label="Open navigation" className="-ml-1.5 md:hidden">
        <PanelLeft />
      </Button>
    </DialogTrigger>
  )
}
