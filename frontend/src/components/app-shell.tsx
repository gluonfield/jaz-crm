import { useNavigate } from '@tanstack/react-router'
import { type ReactNode } from 'react'
import { Toaster } from 'sonner'
import { embedded } from '@/lib/api'
import { signOut } from '@/lib/account'
import { ShortcutProvider, useShortcutActions } from '@/lib/shortcuts'
import { ShortcutsHelp } from './shortcuts-help'
import { getUI, setUI } from '@/lib/ui'
import { CommandPalette } from './command-palette'
import { DesktopSidebar, NavDrawer } from './nav-drawer'
import { useChanges } from '@/lib/changes'
import { useWorkspaceChanges } from '@/lib/workspaces'

export function AppShell({ children }: { children: ReactNode }) {
  return <ShortcutProvider><Shell>{children}</Shell></ShortcutProvider>
}

function Shell({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  useChanges(useWorkspaceChanges())
  const go = (to: string) => () => void navigate({ to })
  useShortcutActions({
    palette: () => setUI({ paletteOpen: !getUI().paletteOpen }),
    help: () => setUI({ helpOpen: true }),
    search: () => setUI({ paletteOpen: true }),
    people: go('/o/people'),
    companies: go('/o/companies'),
    inbox: go('/triage'),
    mine: go('/o/follow_ups'),
    trash: go('/trash'),
    settings: go('/settings'),
    connections: go('/connections'),
    ...(!embedded() && { signOut: () => { void signOut() } }),
  }, 0)

  return (
    <NavDrawer>
      <div className="flex h-full min-h-0">
        <DesktopSidebar />
        <main className="flex min-w-0 flex-1 flex-col overflow-hidden bg-bg pb-[var(--safe-area-bottom)] md:my-2 md:mr-2 md:rounded-[var(--radius-card)]">{children}</main>
        <CommandPalette />
        <ShortcutsHelp />
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
