import { useMutation, useQuery } from '@tanstack/react-query'
import { embedded, rest } from './api'

// The session-only account endpoints; the MCP App has no session and hides them.

export type Me = { name: string; email: string; admin: boolean; workspace: { id: string; name: string } }
export type APIKey = { id: string; label: string; hint: string; createdAt: string }
export type Grant = { id: string; clientName: string; createdAt: string; lastUsedAt: string }

export function useMe() {
  return useQuery({ queryKey: ['me'], queryFn: () => rest<Me>('GET', '/auth/me'), enabled: !embedded(), staleTime: Infinity }).data
}

export function useAPIKeys() {
  return useQuery({ queryKey: ['api-keys'], queryFn: () => rest<APIKey[]>('GET', '/auth/api-keys') }).data ?? []
}

export function useCreateAPIKey() {
  return useMutation({ mutationFn: (label: string) => rest<{ key: string; apiKey: APIKey }>('POST', '/auth/api-keys', { label }) })
}

export function useGrants() {
  return useQuery({ queryKey: ['grants'], queryFn: () => rest<Grant[]>('GET', '/auth/grants') }).data ?? []
}

// useRevoke deletes an API key or an authorized application's grant.
export function useRevoke() {
  return useMutation({ mutationFn: (path: string) => rest('DELETE', path) })
}

export async function signOut() {
  await rest('POST', '/auth/logout')
  window.location.assign('/auth/login')
}
