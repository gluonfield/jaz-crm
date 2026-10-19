import { useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { embedded } from './api'

// useChanges refetches what is on screen whenever the workspace's data
// changes, whoever changed it: the server streams an event per change. A
// stream that reopens may have missed changes, so the page refetches then
// too; one that fails opens again after a growing wait. A hidden tab only
// marks its data stale and refetches when shown. The MCP App has no session
// to stream with, so it refreshes every half minute.
export function useChanges(workspace: string | undefined) {
  const client = useQueryClient()
  useEffect(() => {
    const refresh = () => void client.invalidateQueries({ refetchType: document.hidden ? 'none' : 'active' })
    if (embedded()) {
      const timer = setInterval(refresh, 30_000)
      return () => clearInterval(timer)
    }
    if (!workspace) {
      return
    }
    let source: EventSource
    let retry: number
    let wait = 5000
    const open = (opened: boolean) => {
      source = new EventSource('/api/changes')
      source.onmessage = refresh
      source.onopen = () => {
        wait = 5000
        if (opened) {
          refresh()
        }
        opened = true
      }
      source.onerror = () => {
        if (source.readyState === EventSource.CLOSED) {
          retry = window.setTimeout(() => open(true), wait)
          wait = Math.min(wait * 2, 60_000)
        }
      }
    }
    open(false)
    return () => {
      source.close()
      clearTimeout(retry)
    }
  }, [workspace, client])
}
