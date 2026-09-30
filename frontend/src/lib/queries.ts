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
      onSuccess: () => client.invalidateQueries(),
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

export function useRecords(object: string, query = '', limit = 100, where: Record<string, string> = {}) {
  return useTool<{ records: CrmRecord[] }>('search_records', { object, query, limit, where }, { placeholderData: (previous) => previous })
}

export function useWrite(record: CrmRecord) {
  const upsert = useAction<{ object: string; record_id: string; values?: Record<string, string>; remove?: Record<string, string[]> }>('upsert_record')
  return {
    pending: upsert.isPending,
    set: (slug: string, value: string) => upsert.mutate({ object: record.object, record_id: record.id, values: { [slug]: value } }),
    remove: (slug: string, values: string[]) => upsert.mutate({ object: record.object, record_id: record.id, remove: { [slug]: values } }),
  }
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

// useUpcoming lists a record's scheduled interactions, soonest first.
export function useUpcoming(recordId: string) {
  return useTool<{ interactions: Interaction[] }>('list_interactions', { record_id: recordId, upcoming: true, limit: 5 }).data?.interactions ?? []
}

// useTimeline pages back through a record's interactions of some kinds, or
// of every kind.
export function useTimeline(recordId: string, kinds?: string[]) {
  return useInfiniteQuery({
    queryKey: ['list_interactions', recordId, kinds],
    queryFn: ({ pageParam }) => call<{ interactions: Interaction[] }>('list_interactions', { record_id: recordId, kinds, before: pageParam, limit: pageSize }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => (last.interactions.length === pageSize ? last.interactions[pageSize - 1].started_at : undefined),
  })
}
