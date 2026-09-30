import { useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useTool } from './queries'
import type { Connection, Connections } from './types'

// steps names the sync's steps for people.
export const steps: Record<string, string> = {
  Aliases: 'Checking your addresses',
  GmailBackfill: 'Importing mail',
  GmailIncremental: 'Checking new mail',
  CalendarSync: 'Syncing calendar',
  Triage: 'Sorting contacts',
  FetchContent: 'Fetching conversations',
  Photos: 'Fetching pictures and logos',
  Watch: 'Checking for updates',
  DueMeetings: 'Checking meetings',
}

// syncing reports whether a connection is working now or still importing.
export const syncing = (c: Connection) => c.status === 'active' && (!c.backfilled || !!c.step)

// useConnections polls the workspace's connections, every few seconds while
// one syncs and every half minute otherwise.
export function useConnections() {
  return useTool<Connections>('list_connections', {}, { refetchInterval: (query) => (query.state.data?.connections.some(syncing) ? 3000 : 30_000) }).data
}

// useLiveWhileSyncing refreshes what is on screen while mail arrives.
export function useLiveWhileSyncing() {
  const busy = useConnections()?.connections.some(syncing) ?? false
  const client = useQueryClient()
  useEffect(() => {
    if (!busy) {
      return
    }
    const timer = setInterval(() => void client.invalidateQueries({ predicate: (q) => q.queryKey[0] !== 'list_connections' }), 5000)
    return () => clearInterval(timer)
  }, [busy, client])
}
