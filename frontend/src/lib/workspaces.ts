import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { call } from './api'
import { useTool } from './queries'

export type WorkspaceRef = { id: string; name: string; current?: boolean }

// The workspace list is checked this often, as an agent sharing this sign-in
// can switch workspaces too.
const checkEvery = 10_000

export function useWorkspaces() {
  return useTool<{ workspaces: WorkspaceRef[] }>('list_workspaces', {}, { refetchInterval: checkEvery }).data?.workspaces ?? []
}

// useMoveWorkspace switches to, or creates, a workspace; the refetch every
// change triggers lets useWorkspaceChanges start over.
export function useMoveWorkspace() {
  return useMutation({
    mutationFn: (to: { workspace_id: string } | { name: string }) => call<WorkspaceRef>('workspace_id' in to ? 'switch_workspace' : 'create_workspace', to),
  })
}

// useWorkspaceChanges starts over on People whenever the current workspace
// changes, switched here or elsewhere, so nothing from the last one lingers.
// Going through "/" would load the whole page again.
export function useWorkspaceChanges() {
  const current = useWorkspaces().find((w) => w.current)?.id
  const seen = useRef(current)
  const client = useQueryClient()
  const navigate = useNavigate()
  useEffect(() => {
    if (!current || current === seen.current) {
      return
    }
    const first = seen.current === undefined
    seen.current = current
    if (!first) {
      void client.resetQueries()
      void navigate({ to: '/o/$object', params: { object: 'people' } })
    }
  }, [current, client, navigate])
}
