import { useNavigate } from '@tanstack/react-router'
import { type ReactNode, useEffect } from 'react'
import { Toaster } from 'sonner'
import { isTyping } from '@/lib/hooks'
import { getUI, setUI } from '@/lib/ui'
import { CommandPalette } from './command-palette'
import { NavDrawer } from './nav-drawer'
import { Sidebar } from './sidebar'
import { useLiveSync } from '@/lib/sync'
import { useWorkspaceChanges } from '@/lib/workspaces'

const goKeys: Record<string, string> = { p: '/o/people', c: '/o/companies', t: '/triage', x: '/connections', s: '/settings' }

export function AppShell({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  useLiveSync()
  useWorkspaceChanges()
  useEffect(() => {
    let pendingG = false
    const onKey = (e: KeyboardEvent) => {
      if (e.defaultPrevented) {
        return
      }
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setUI({ paletteOpen: !getUI().paletteOpen })
        return
      }
      if (isTyping(e) || e.metaKey || e.ctrlKey || e.altKey || getUI().paletteOpen) {
        return
      }
      if (pendingG) {
        pendingG = false
        if (goKeys[e.key]) {
          e.preventDefault()
          navigate({ to: goKeys[e.key] })
        }
      } else if (e.key === 'g') {
        pendingG = true
        window.setTimeout(() => (pendingG = false), 1200)
      } else if (e.key === '/') {
        e.preventDefault()
        // A page's own search takes /, as on most sites; elsewhere / searches everything.
        const search = document.querySelector<HTMLInputElement>('[data-page-search]')
        if (search) {
          search.focus()
        } else {
          setUI({ paletteOpen: true })
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [navigate])

  return (
    <NavDrawer>
      <div className="flex h-full min-h-0">
        <Sidebar className="max-md:hidden" />
        <main className="flex min-w-0 flex-1 flex-col overflow-hidden bg-bg pb-[var(--safe-area-bottom)] md:my-2 md:mr-2 md:rounded-[var(--radius-card)]">{children}</main>
        <CommandPalette />
        <Toaster
          position="bottom-left"
          mobileOffset={{ bottom: 'calc(16px + var(--safe-area-bottom))' }}
          toastOptions={{
            unstyled: true,
            classNames: {
              toast:
                'flex w-[340px] items-center gap-2.5 rounded-[var(--radius-card)] border border-border bg-raised px-3.5 py-3 text-[13px] text-ink shadow-[var(--shadow-raised)]',
              title: 'font-medium',
              description: 'text-ink-3',
            },
          }}
        />
      </div>
    </NavDrawer>
  )
}
