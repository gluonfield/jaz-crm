import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { Check, Inbox, Search, X } from 'lucide-react'
import { useState } from 'react'
import { Button, Header, Tab, inputClass } from '@/components/controls'
import { EmptyState } from '@/components/empty-state'
import { RecordIcon } from '@/components/icons'
import { Kbd } from '@/components/kbd'
import { timeAgo } from '@/lib/format'
import { useDebounced, useListKeys } from '@/lib/hooks'
import { useAction, useTool } from '@/lib/queries'
import type { Contact, Verdict } from '@/lib/types'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/_app/triage')({ component: TriagePage })

const tabs: { status: Verdict; label: string }[] = [
  { status: 'pending', label: 'Pending' },
  { status: 'kept', label: 'Kept' },
  { status: 'skipped', label: 'Skipped' },
]

const deciders: Record<string, string> = { rule: 'Rule', agent: 'Agent', user: 'You' }

function TriagePage() {
  const [status, setStatus] = useState<Verdict>('pending')
  const [search, setSearch] = useState('')
  const contacts = useTool<{ contacts: Contact[] }>('list_triage', { status, query: useDebounced(search), limit: 200 }, { placeholderData: (p) => p }).data?.contacts
  const decide = useAction<object>('decide_triage')
  const navigate = useNavigate()
  const choose = (contact: Contact, decision: 'keep' | 'skip') => decide.mutate({ addresses: [contact.address], decision })
  const [focus] = useListKeys(contacts?.length ?? 0, {
    y: (i) => contacts && choose(contacts[i], 'keep'),
    n: (i) => contacts && choose(contacts[i], 'skip'),
    Enter: (i) => contacts?.[i].person_id && navigate({ to: '/r/$recordId', params: { recordId: contacts[i].person_id } }),
  })
  return (
    <>
      <Header>
        <Inbox />
        Triage
        <div className="ml-3 flex gap-1">
          {tabs.map((t) => (
            <Tab key={t.status} active={status === t.status} onClick={() => setStatus(t.status)}>
              {t.label}
            </Tab>
          ))}
        </div>
        <label className="relative ml-auto flex items-center">
          <Search className="pointer-events-none absolute left-2 size-3.5 text-ink-3" />
          <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search" aria-label="Search addresses" className={cn(inputClass, 'w-56 pl-7')} />
        </label>
      </Header>
      {contacts?.length === 0 ? (
        <EmptyState title={status === 'pending' ? 'Nothing to triage' : `No ${status} addresses`} icon={<Inbox />} />
      ) : (
        <ul className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto py-1">
          {contacts?.map((c, index) => {
            const domain = c.address.split('@')[1]
            return (
              <li
                key={c.address}
                data-row={index}
                className={cn('group flex h-12 items-center gap-3 border-b border-border/50 px-4 text-[13px]', focus === index && 'bg-list-hover')}
              >
                <RecordIcon object="people" name={c.name || c.address} photo={c.photo} size={24} />
                <div className="min-w-0 flex-1">
                  <div className="flex items-baseline gap-2">
                    {c.person_id ? (
                      <Link to="/r/$recordId" params={{ recordId: c.person_id }} className="truncate font-medium text-ink hover:underline">
                        {c.name || c.address}
                      </Link>
                    ) : (
                      <span className="truncate font-medium text-ink">{c.name || c.address}</span>
                    )}
                    {c.name && <span className="truncate text-ink-3">{c.address}</span>}
                  </div>
                  <div className="truncate text-[12px] text-ink-3">
                    {c.interactions} conversation{c.interactions === 1 ? '' : 's'} · {timeAgo(c.last_seen)}
                    {(c.decided_by || c.reason) && ` · ${[deciders[c.decided_by ?? ''], c.reason].filter(Boolean).join(': ')}`}
                  </div>
                </div>
                <div className="flex items-center gap-1.5 opacity-60 group-hover:opacity-100">
                  {status !== 'kept' && (
                    <Button onClick={() => choose(c, 'keep')}>
                      <Check /> Keep {focus === index && <Kbd>Y</Kbd>}
                    </Button>
                  )}
                  {status !== 'skipped' && (
                    <Button onClick={() => choose(c, 'skip')}>
                      <X /> Skip {focus === index && <Kbd>N</Kbd>}
                    </Button>
                  )}
                  {domain && status !== 'skipped' && (
                    <Button onClick={() => decide.mutate({ domains: [domain], decision: 'skip' })} title={`Skip everyone at ${domain}, now and later`}>
                      Skip {domain}
                    </Button>
                  )}
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </>
  )
}
