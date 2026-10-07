import { useNavigate } from '@tanstack/react-router'
import { Command as CommandPrimitive } from 'cmdk'
import { Inbox, Moon, PlugZap, Settings, Sun } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { recordName } from '@/lib/crm'
import { pageIcon } from '@/lib/pages'
import { useDebounced } from '@/lib/hooks'
import { useObjects, useRecordSearch } from '@/lib/queries'
import { setSchemePreference } from '@/lib/theme'
import { setUI, useUI } from '@/lib/ui'
import { ObjectIcon, RecordIcon } from './icons'
import { Kbd } from './kbd'

export function CommandPalette() {
  const open = useUI((s) => s.paletteOpen)
  return (
    <Dialog open={open} onOpenChange={(next) => setUI({ paletteOpen: next })}>
      <DialogContent
        showCloseButton={false}
        className="top-[14%] w-[640px] max-w-[calc(100vw-2rem)] translate-y-0 gap-0 overflow-hidden rounded-[12px] border-border bg-raised p-0 shadow-[var(--shadow-raised)] sm:max-w-[640px]"
      >
        <DialogTitle className="sr-only">Command menu</DialogTitle>
        {open && <Palette />}
      </DialogContent>
    </Dialog>
  )
}

function Palette() {
  const navigate = useNavigate()
  const objects = useObjects() ?? []
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState('')
  const query = useDebounced(search.trim())
  const records = useRecordSearch(query)
  const [top, setTop] = useState<string>()
  if (records[0]?.id !== top) {
    setTop(records[0]?.id)
    if (records[0]) setSelected(records[0].id)
  }
  const go = (to: string) => {
    setUI({ paletteOpen: false })
    navigate({ to })
  }
  const matches = (label: string) => label.toLowerCase().includes(search.trim().toLowerCase())
  const pages = [
    ...objects.map((o) => ({ label: o.name, to: `/o/${o.slug}`, icon: <ObjectIcon slug={o.slug} />, shortcut: undefined })),
    { label: 'Triage', to: '/triage', icon: <Inbox />, shortcut: 'G T' },
    { label: 'Connections', to: '/connections', icon: <PlugZap />, shortcut: 'G X' },
    { label: 'Settings', to: '/settings', icon: <Settings />, shortcut: 'G S' },
  ].filter((p) => matches(p.label))
  const themes = [
    { label: 'Light theme', scheme: 'light' as const, icon: <Sun /> },
    { label: 'Dark theme', scheme: 'dark' as const, icon: <Moon /> },
  ].filter((s) => matches(s.label))
  return (
    <CommandPrimitive loop shouldFilter={false} label="Search" value={selected} onValueChange={setSelected}>
      <CommandPrimitive.Input
        asChild
        autoFocus
        value={search}
        onValueChange={setSearch}
        placeholder="Search people, companies and records..."
        className="h-12 w-full border-b border-border bg-transparent px-4 text-[15px] text-ink outline-none placeholder:text-ink-3"
      >
        <input type="search" />
      </CommandPrimitive.Input>
      <CommandPrimitive.List className="scrollbar-quiet max-h-[min(420px,60vh)] overflow-y-auto p-1.5">
        {records.length > 0 && (
          <CommandPrimitive.Group heading="Records" className={groupClass}>
            {records.map((r) => (
              <Item key={r.id} value={r.id} onSelect={() => go(`/r/${r.id}`)} icon={<RecordIcon object={r.object} name={recordName(r)} photo={r.photo} icon={pageIcon(r)} />}>
                {recordName(r)}
                <span className="ml-2 text-ink-3">{objects.find((o) => o.slug === r.object)?.name}</span>
              </Item>
            ))}
          </CommandPrimitive.Group>
        )}
        {pages.length > 0 && (
          <CommandPrimitive.Group heading="Go to" className={groupClass}>
            {pages.map((p) => (
              <Item key={p.to} onSelect={() => go(p.to)} icon={p.icon} shortcut={p.shortcut}>
                {p.label}
              </Item>
            ))}
          </CommandPrimitive.Group>
        )}
        {themes.length > 0 && (
          <CommandPrimitive.Group heading="Theme" className={groupClass}>
            {themes.map((s) => (
              <Item key={s.scheme} onSelect={() => setSchemePreference(s.scheme)} icon={s.icon}>
                {s.label}
              </Item>
            ))}
          </CommandPrimitive.Group>
        )}
      </CommandPrimitive.List>
    </CommandPrimitive>
  )
}

const groupClass = '[&_[cmdk-group-heading]]:px-2.5 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:text-[12px] [&_[cmdk-group-heading]]:text-ink-3'

function Item({ value, icon, shortcut, onSelect, children }: { value?: string; icon: ReactNode; shortcut?: string; onSelect: () => void; children: ReactNode }) {
  return (
    <CommandPrimitive.Item
      value={value}
      onSelect={onSelect}
      className="flex h-9 cursor-default items-center gap-3 rounded-[7px] px-2.5 text-[13.5px] text-ink outline-none data-[selected=true]:bg-list-active [&>svg]:size-4 [&>svg]:shrink-0 [&>svg]:text-ink-2"
    >
      {icon}
      <span className="flex min-w-0 flex-1 items-center truncate">{children}</span>
      {shortcut && <Kbd>{shortcut}</Kbd>}
    </CommandPrimitive.Item>
  )
}
