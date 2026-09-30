import { useNavigate } from '@tanstack/react-router'
import { type ReactNode, useEffect } from 'react'
import { Toaster } from 'sonner'
import { isTyping } from '@/lib/hooks'
import { getUI, setUI } from '@/lib/ui'
import { CommandPalette } from './command-palette'
import { Sidebar } from './sidebar'
import { useLiveWhileSyncing } from '@/lib/sync'

const goKeys: Record<string, string> = { p: '/o/people', c: '/o/companies', t: '/triage', f: '/search', x: '/connections', s: '/settings' }

export function AppShell({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  useLiveWhileSyncing()
  useEffect(() => {
    let pendingG = false
    const onKey = (e: KeyboardEvent) => {
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
        setUI({ paletteOpen: true })
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [navigate])

  return (
    <div className="flex h-full min-h-0">
      <Sidebar />
      <main className="my-2 mr-2 flex min-w-0 flex-1 flex-col overflow-hidden rounded-[var(--radius-card)] bg-bg">{children}</main>
      <CommandPalette />
      <Toaster
        position="bottom-left"
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
  )
}
