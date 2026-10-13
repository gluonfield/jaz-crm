import { Button } from '@jaz/ui/button'
import { useRouter } from '@tanstack/react-router'
import { PanelLeft } from 'lucide-react'
import { Dialog as DialogPrimitive } from 'radix-ui'
import { type PointerEvent, type ReactNode, useEffect, useRef, useState } from 'react'
import { Dialog, DialogOverlay, DialogPortal, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { usePhone } from '@/lib/hooks'
import { getUI, setUI, useUI } from '@/lib/ui'
import { cn } from '@/lib/utils'
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
  const phone = usePhone()
  const collapsed = useUI((ui) => ui.sidebarCollapsed)
  if (!phone) {
    return collapsed && <Button variant="ghost" size="icon" aria-label="Expand sidebar" title="Expand sidebar" className="-ml-1.5" onClick={() => setUI({ sidebarCollapsed: false })}><PanelLeft /></Button>
  }
  return (
    <DialogTrigger asChild>
      <Button variant="ghost" size="icon" aria-label="Open navigation" className="-ml-1.5 md:hidden">
        <PanelLeft />
      </Button>
    </DialogTrigger>
  )
}

export function DesktopSidebar() {
  const width = useUI((ui) => ui.sidebarWidth)
  const collapsed = useUI((ui) => ui.sidebarCollapsed)
  const drag = useRef<{ x: number; width: number } | null>(null)
  const resize = (next: number) => setUI({ sidebarWidth: Math.round(Math.max(200, Math.min(420, next))) })
  const move = (e: PointerEvent<HTMLDivElement>) => {
    if (drag.current && e.currentTarget.hasPointerCapture(e.pointerId)) {
      resize(drag.current.width + e.clientX - drag.current.x)
    }
  }
  return (
    <div style={{ width }} className={cn('relative hidden shrink-0 md:block', collapsed && 'md:hidden')}>
      <Sidebar className="h-full w-full overflow-y-auto" />
      <div
        role="separator"
        tabIndex={0}
        aria-label="Resize sidebar"
        aria-orientation="vertical"
        aria-valuemin={200}
        aria-valuemax={420}
        aria-valuenow={width}
        onPointerDown={(e) => {
          if (e.button !== 0) {
            return
          }
          e.preventDefault()
          drag.current = { x: e.clientX, width: e.currentTarget.parentElement!.getBoundingClientRect().width }
          e.currentTarget.setPointerCapture(e.pointerId)
          e.currentTarget.focus()
        }}
        onPointerMove={move}
        onPointerUp={move}
        onLostPointerCapture={() => (drag.current = null)}
        onDoubleClick={() => resize(232)}
        onKeyDown={(e) => {
          if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
            e.preventDefault()
            resize(getUI().sidebarWidth + (e.key === 'ArrowLeft' ? -16 : 16))
          } else if (e.key === 'Home') {
            e.preventDefault()
            resize(232)
          }
        }}
        className="absolute inset-y-0 -right-1 z-10 w-2 cursor-col-resize touch-none select-none outline-none after:absolute after:inset-y-2 after:left-1/2 after:w-px after:transition-colors hover:after:bg-border focus-visible:after:bg-primary active:after:bg-primary"
      />
    </div>
  )
}
