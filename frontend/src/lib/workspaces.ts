import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { call } from './api'
import { useTool } from './queries'

export type WorkspaceRef = { id: string; name: string; default?: boolean }

export function useWorkspaces() {
  return useTool<{ workspaces: WorkspaceRef[] }>('list_workspaces').data?.workspaces ?? []
}

// useMoveWorkspace makes another workspace, or a new one, the default; the
// refetch every change triggers lets useWorkspaceChanges start over.
export function useMoveWorkspace() {
  return useMutation({
    mutationFn: async (to: { workspace_id: string } | { name: string }) => {
      const id = 'workspace_id' in to ? to.workspace_id : (await call<WorkspaceRef>('create_workspace', to)).id
      return call<WorkspaceRef>('switch_workspace', { workspace_id: id })
    },
  })
}

// useWorkspaceChanges starts over on People whenever the current workspace
// changes, switched here or elsewhere, so nothing from the last one lingers,
// and returns the current workspace. Going through "/" would load the whole
// page again.
export function useWorkspaceChanges() {
  const current = useWorkspaces().find((w) => w.default)?.id
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
  return current
}
