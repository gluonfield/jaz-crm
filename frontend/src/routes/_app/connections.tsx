import { createFileRoute } from '@tanstack/react-router'
import { PlugZap } from 'lucide-react'
import { Button } from '@jaz/ui/button'
import { Header, Row, Section } from '@/components/controls'
import { formatDate, timeAgo } from '@/lib/format'
import { useAction } from '@/lib/queries'
import { steps, useConnect, useConnections } from '@/lib/sync'
import type { Connection } from '@/lib/types'

export const Route = createFileRoute('/_app/connections')({
  validateSearch: (search: Record<string, unknown>) => ({ error: typeof search.error === 'string' ? search.error : undefined }),
  component: ConnectionsPage,
})

const streams: Record<string, string> = { gmail_history: 'Mail', calendar: 'Calendar' }

// progress says what a connection is doing and how much mail is in.
function progress(c: Connection, since: string) {
  if (c.status === 'revoked') {
    return ['Access revoked; connect again', '']
  }
  const messages = `${c.messages.toLocaleString('en')} messages`
  if (!c.backfilled) {
    const reached = c.oldest ? ` · reached ${formatDate(c.oldest)}` : ''
    return [steps[c.step ?? ''] ?? steps.GmailBackfill, `${messages} so far${reached}, going back to ${formatDate(since)}`]
  }
  const synced = Object.entries(streams).map(([stream, label]) => `${label} ${c.synced[stream] ? `synced ${timeAgo(c.synced[stream])}` : 'syncing'}`)
  return [c.step ? steps[c.step] : 'Up to date', [messages, ...synced].join(' · ')]
}

function ConnectionsPage() {
  const { error } = Route.useSearch()
  const data = useConnections()
  const connect = useConnect()
  const disconnect = useAction<object>('disconnect')
  return (
    <>
      <Header>
        <PlugZap />
        Connections
        {connect && (
          <Button variant="primary" className="ml-auto" onClick={connect}>
            Connect Google
          </Button>
        )}
      </Header>
      <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-[680px] px-10 pt-9">
          {error && <p className="mb-6 rounded-[var(--radius-card)] border border-danger/40 bg-danger-soft px-4 py-3 text-[13px] text-danger">{error}</p>}
          <Section title="Google accounts">
            {data?.connections.length === 0 && (
              <Row className="text-ink-3">
                {connect ? `Connect a Google account to import its mail and meetings since ${formatDate(data.since)}.` : 'Google is not configured on this server.'}
              </Row>
            )}
            {data?.connections.map((c) => {
              const [now, detail] = progress(c, data.since)
              return (
                <Row key={c.id} className="items-start">
                  <div className="min-w-0 flex-1">
                    <div className="truncate font-medium text-ink">{c.account}</div>
                    <div className="truncate text-[12px] text-ink-2">{now}</div>
                    <div className="truncate text-[12px] text-ink-3">{detail}</div>
                    {c.status === 'active' && !c.backfilled && (
                      <div role="progressbar" aria-label="Importing mail" className="mt-2 h-1 overflow-hidden rounded-full bg-list-active">
                        <div className="h-full w-1/3 animate-sweep rounded-full bg-primary motion-reduce:animate-none" />
                      </div>
                    )}
                  </div>
                  <Button onClick={() => disconnect.mutate({ connection_id: c.id })}>Disconnect</Button>
                </Row>
              )
            })}
          </Section>
        </div>
      </div>
    </>
  )
}
