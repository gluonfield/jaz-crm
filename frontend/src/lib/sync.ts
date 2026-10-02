import { useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { embedded } from './api'
import { app } from './mcp-app'
import { useTool } from './queries'
import type { Connection, Connections } from './types'

// steps names the sync's steps for people.
export const steps: Record<string, string> = {
  Aliases: 'Checking your addresses',
  GmailBackfill: 'Importing mail',
  GmailIncremental: 'Checking new mail',
  CalendarSync: 'Syncing calendar',
  Triage: 'Sorting contacts',
  CompanyLogos: 'Fetching company logos',
  FetchContent: 'Fetching conversations',
  FollowUps: 'Preparing replies',
  Photos: 'Fetching profile pictures',
  Watch: 'Checking for updates',
  DueMeetings: 'Checking meetings',
}

// syncing reports whether a connection is working now or still importing.
export const syncing = (c: Connection) => c.status === 'active' && (!c.backfilled || !!c.step)

// useConnections polls the workspace's connections, every few seconds while
// one syncs and every half minute otherwise.
export function useConnections(interval?: number) {
  return useTool<Connections>('list_connections', {}, { refetchInterval: (query) => interval ?? (query.state.data?.connections.some(syncing) ? 3000 : 30_000) }).data
}

// useMail says whether any mail is connected and whether it is still
// importing, once the connections are known.
export function useMail() {
  const active = useConnections()?.connections.filter((c) => c.status === 'active')
  if (!active) {
    return undefined
  }
  if (active.length === 0) {
    return 'none'
  }
  return active.some((c) => !c.backfilled) ? 'importing' : 'ready'
}

// useConnect starts connecting a Google account, when the server can.
export function useConnect() {
  const url = useConnections()?.connect_url
  if (!url) {
    return undefined
  }
  return () => {
    if (embedded()) {
      void app.openLink({ url })
    } else {
      window.location.assign(url)
    }
  }
}

// useLiveSync refreshes on progress changes, including passes completed
// between polls.
export function useLiveSync() {
  const connections = useConnections()?.connections
  const client = useQueryClient()
  useEffect(() => {
    if (!connections?.some((c) => c.status === 'active')) {
      return
    }
    const refresh = () => void client.invalidateQueries({ predicate: (q) => q.queryKey[0] !== 'list_connections' })
    refresh()
    if (!connections.some(syncing)) {
      return
    }
    const timer = setInterval(refresh, 5000)
    return () => clearInterval(timer)
  }, [connections, client])
}
