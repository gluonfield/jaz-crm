import { Link, Outlet, createFileRoute } from '@tanstack/react-router'
import { Settings } from 'lucide-react'
import { Header } from '@/components/controls'
import { NavButton } from '@/components/nav-drawer'

export const Route = createFileRoute('/_app/settings')({ component: SettingsPage })

const pages = [
  { to: '/settings', label: 'General' },
  { to: '/settings/triage', label: 'Triage' },
  { to: '/settings/team', label: 'Team' },
  { to: '/settings/schema', label: 'Schema' },
  { to: '/settings/drafting', label: 'Drafting' },
  { to: '/settings/access', label: 'Access' },
] as const

function SettingsPage() {
  return (
    <>
      <Header><NavButton /><Settings /> Settings</Header>
      <nav aria-label="Settings" className="scrollbar-quiet flex shrink-0 gap-1 overflow-x-auto border-b border-border px-3">
        {pages.map(({ to, label }) => (
          <Link key={to} to={to} activeOptions={{ exact: true }}
            className="flex min-h-10 shrink-0 items-center border-b-2 px-3 text-[13px] outline-none hover:text-ink focus-visible:bg-list-hover"
            inactiveProps={{ className: 'border-transparent text-ink-3' }}
            activeProps={{ className: 'border-primary font-medium text-ink', 'aria-current': 'page' }}>
            {label}
          </Link>
        ))}
      </nav>
      <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-[760px] px-4 pb-12 pt-7 sm:px-8"><Outlet /></div>
      </div>
    </>
  )
}
