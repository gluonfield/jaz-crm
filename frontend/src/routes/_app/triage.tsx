import { useMutation } from '@tanstack/react-query'
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { Check, Inbox, Search, X } from 'lucide-react'
import { useState } from 'react'
import { Button, Header, Tab, inputClass } from '@/components/controls'
import { ConnectGoogle, EmptyState } from '@/components/empty-state'
import { RecordIcon } from '@/components/icons'
import { Kbd } from '@/components/kbd'
import { call } from '@/lib/api'
import { timeAgo } from '@/lib/format'
import { useDebounced, useListKeys } from '@/lib/hooks'
import { useTool } from '@/lib/queries'
import { useMail } from '@/lib/sync'
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
  const query = useDebounced(search.trim())
  const listed = useTool<{ contacts: Contact[] }>('list_triage', { status, query, limit: 200 }, { placeholderData: (p) => p }).data?.contacts
  const navigate = useNavigate()
  // A decided address or domain leaves the list at once and returns if the
  // decision fails. The decision settles no sooner than the row's 300ms exit,
  // so the refetch that unmounts the row cannot cut its animation short.
  const decide = useMutation({ mutationFn: (args: object) => Promise.all([call('decide_triage', args), new Promise((done) => setTimeout(done, 300))]) })
  const [leaving, setLeaving] = useState<string[]>([])
  const leave = (key: string, args: object) => {
    setLeaving((keys) => [...keys, key])
    decide
      .mutateAsync(args)
      .catch(() => {})
      .finally(() => setLeaving((keys) => keys.filter((k) => k !== key)))
  }
  const contacts = listed?.filter((c) => !leaving.some((key) => c.address === key || c.address.endsWith(`@${key}`))) ?? []
  const choose = (contact: Contact, decision: 'keep' | 'skip') => leave(contact.address, { addresses: [contact.address], decision })
  const [focus] = useListKeys(contacts.length, {
    y: (i) => choose(contacts[i], 'keep'),
    n: (i) => choose(contacts[i], 'skip'),
    Enter: (i) => contacts[i].person_id && navigate({ to: '/r/$recordId', params: { recordId: contacts[i].person_id } }),
  })
  return (
    <>
      <Header>
        <Inbox />
        Triage
        <div className="ml-3 flex gap-1.5">
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
      {listed?.length === 0 ? (
        <Empty status={status} query={query} />
      ) : (
        <ul className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto py-1">
          {listed?.map((c) => {
            const domain = c.address.split('@')[1]
            const index = contacts.indexOf(c)
            const focused = index >= 0 && focus === index
            return (
              <li
                key={c.address}
                data-row={index}
                className={cn('leavable group flex h-12 items-center gap-3 border-b border-border/50 px-4 text-[13px]', index < 0 && 'leaving', focused && 'bg-list-hover')}
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
                      <Check /> Keep {focused && <Kbd>Y</Kbd>}
                    </Button>
                  )}
                  {status !== 'skipped' && (
                    <Button onClick={() => choose(c, 'skip')}>
                      <X /> Skip {focused && <Kbd>N</Kbd>}
                    </Button>
                  )}
                  {domain && status !== 'skipped' && (
                    <Button onClick={() => leave(domain, { domains: [domain], decision: 'skip' })} title={`Skip everyone at ${domain}, now and later`}>
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

function Empty({ status, query }: { status: Verdict; query: string }) {
  const mail = useMail()
  if (query) {
    return <EmptyState title={`No ${status} addresses match “${query}”`} icon={<Search />} />
  }
  if (status === 'kept') {
    return (
      <EmptyState title="No one kept yet" icon={<Check />}>
        People you keep become records in the CRM, together with their conversations.
      </EmptyState>
    )
  }
  if (status === 'skipped') {
    return (
      <EmptyState title="Nothing skipped" icon={<X />}>
        Addresses you skip, and automated senders, stay out of the CRM along with their mail.
      </EmptyState>
    )
  }
  if (mail === 'none') {
    return (
      <EmptyState title="Nothing to triage" icon={<Inbox />} action={<ConnectGoogle />}>
        Connect Google, and new people who email you land here for you to keep or skip.
      </EmptyState>
    )
  }
  return (
    <EmptyState title={mail === 'importing' ? 'Nothing to triage yet' : 'You’re all caught up'} icon={<Inbox />}>
      New people who email you land here within minutes for you to keep or skip; anyone you write to or meet is kept automatically.
    </EmptyState>
  )
}
