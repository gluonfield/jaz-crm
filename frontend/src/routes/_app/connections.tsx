import { createFileRoute } from '@tanstack/react-router'
import { PlugZap } from 'lucide-react'
import { Button, Header, Row, Section } from '@/components/controls'
import { embedded } from '@/lib/api'
import { timeAgo } from '@/lib/format'
import { app } from '@/lib/mcp-app'
import { useAction, useTool } from '@/lib/queries'
import type { Connection } from '@/lib/types'

export const Route = createFileRoute('/_app/connections')({
  validateSearch: (search: Record<string, unknown>) => ({ error: typeof search.error === 'string' ? search.error : undefined }),
  component: ConnectionsPage,
})

const streams: Record<string, string> = { gmail_history: 'Mail', calendar: 'Calendar' }

function ConnectionsPage() {
  const { error } = Route.useSearch()
  const data = useTool<{ connections: Connection[]; connect_url?: string }>('list_connections').data
  const disconnect = useAction<object>('disconnect')
  const connect = () => {
    if (!data?.connect_url) {
      return
    }
    if (embedded()) {
      void app.openLink({ url: data.connect_url })
    } else {
      window.location.assign(data.connect_url)
    }
  }
  return (
    <>
      <Header>
        <PlugZap />
        Connections
        {data?.connect_url && (
          <Button primary className="ml-auto" onClick={connect}>
            Connect Google
          </Button>
        )}
      </Header>
      <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-[680px] px-10 pt-9">
          {error && <p className="mb-6 rounded-[var(--radius-card)] border border-danger/40 bg-danger-soft px-4 py-3 text-[13px] text-danger">{error}</p>}
          <Section title="Google accounts">
            {data?.connections.length === 0 && (
              <Row className="text-ink-3">{data.connect_url ? 'No accounts connected' : 'Google is not configured on this server'}</Row>
            )}
            {data?.connections.map((c) => (
              <Row key={c.id}>
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium text-ink">{c.account}</div>
                  <div className="truncate text-[12px] text-ink-3">
                    {c.status === 'revoked'
                      ? 'Access revoked; connect again'
                      : Object.entries(streams)
                          .map(([stream, label]) => `${label} ${c.synced[stream] ? `synced ${timeAgo(c.synced[stream])}` : 'syncing'}`)
                          .join(' · ')}
                  </div>
                </div>
                <Button onClick={() => disconnect.mutate({ connection_id: c.id })}>Disconnect</Button>
              </Row>
            ))}
          </Section>
        </div>
      </div>
    </>
  )
}
