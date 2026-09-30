import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { call } from './api'
import { useTool } from './queries'

export type WorkspaceRef = { id: string; name: string; current?: boolean }

export function useWorkspaces() {
  return useTool<{ workspaces: WorkspaceRef[] }>('list_workspaces').data?.workspaces ?? []
}

// useMoveWorkspace switches to, or creates, a workspace, then drops every
// cached query and starts from home so nothing from the last one lingers.
export function useMoveWorkspace() {
  const client = useQueryClient()
  const navigate = useNavigate()
  return useMutation({
    mutationFn: (to: { workspace_id: string } | { name: string }) => call<WorkspaceRef>('workspace_id' in to ? 'switch_workspace' : 'create_workspace', to),
    onSuccess: () => {
      client.clear()
      void navigate({ to: '/' })
    },
  })
}
