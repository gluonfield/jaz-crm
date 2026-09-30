import { MutationCache, QueryClient, type UseQueryOptions, useInfiniteQuery, useMutation, useQueries, useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { call } from './api'
import type { CrmObject, CrmRecord, Interaction, Workspace } from './types'

// Every query is a tool call keyed by [tool, args]. Any successful change
// refetches what is on screen, so views never patch caches by hand, and any
// failed one shows its error.
export function newQueryClient() {
  const client: QueryClient = new QueryClient({
    defaultOptions: { queries: { retry: 1 } },
    mutationCache: new MutationCache({
      onSuccess: () => {
        void client.invalidateQueries()
      },
      onError: (error) => {
        toast.error(error.message)
      },
    }),
  })
  return client
}

export const toolQuery = <T>(tool: string, args: object) => ({ queryKey: [tool, args], queryFn: () => call<T>(tool, args) })

export function useTool<T>(tool: string, args: object = {}, options: Omit<UseQueryOptions<T>, 'queryKey' | 'queryFn'> = {}) {
  return useQuery({ ...toolQuery<T>(tool, args), ...options })
}

export function useAction<A extends object, T = unknown>(tool: string) {
  return useMutation({ mutationFn: (args: A) => call<T>(tool, args) })
}

export function useObjects() {
  return useTool<{ objects: CrmObject[] }>('list_objects', {}, { staleTime: 60_000 }).data?.objects
}

export function useWorkspace() {
  return useTool<Workspace>('get_workspace').data
}

export function useRecords(object: string, query = '', limit = 100) {
  return useTool<{ records: CrmRecord[] }>('search_records', { object, query, limit }, { placeholderData: (previous) => previous })
}

// useRecordSearch finds records of every object by text.
export function useRecordSearch(query: string, limit = 5) {
  const objects = useObjects() ?? []
  return useQueries({
    queries: objects.map((o) => ({ ...toolQuery<{ records: CrmRecord[] }>('search_records', { object: o.slug, query, limit }), enabled: !!query })),
    combine: (results) => results.flatMap((r) => r.data?.records ?? []),
  })
}

const pageSize = 20

// useTimeline pages back through a record's interactions.
export function useTimeline(recordId: string) {
  return useInfiniteQuery({
    queryKey: ['list_interactions', recordId],
    queryFn: ({ pageParam }) => call<{ interactions: Interaction[] }>('list_interactions', { record_id: recordId, before: pageParam, limit: pageSize }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => (last.interactions.length === pageSize ? last.interactions[pageSize - 1].started_at : undefined),
  })
}
