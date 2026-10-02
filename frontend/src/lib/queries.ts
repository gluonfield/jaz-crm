import { MutationCache, QueryClient, type UseQueryOptions, useInfiniteQuery, useMutation, useQueries, useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { toast } from 'sonner'
import { call } from './api'
import type { CrmObject, CrmRecord, Interaction, RecordFilter, Relation, Workspace } from './types'

// recordWrites change only records and their schema, which recordReads show.
const recordWrites = new Set(['upsert_record', 'add_attribute_option', 'edit_pipeline_stage', 'create_object', 'create_attribute', 'edit_object', 'edit_attribute'])
const recordReads = new Set(['search_records', 'get_record', 'get_draft_sender', 'list_objects', 'record_history', 'list_saved_filters', 'get_active_filter'])

// Every query is a tool call keyed by [tool, args]. A successful change
// refetches what it can alter on screen, everything unless it only writes
// records, so views never patch caches by hand; any failed one shows its
// error. Data read moments ago is reused on the way back to it.
export function newQueryClient() {
  const client: QueryClient = new QueryClient({
    defaultOptions: { queries: { retry: 1, staleTime: 10_000 } },
    mutationCache: new MutationCache({
      onSuccess: (_data, _variables, _context, mutation) =>
        client.invalidateQueries(recordWrites.has(String(mutation.options.mutationKey?.[0])) ? { predicate: (query) => recordReads.has(String(query.queryKey[0])) } : undefined),
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

// useAction runs a tool. A removal passes how long its exit animation runs, so
// the refetch that drops the element cannot cut the animation short.
export function useAction<A extends object, T = unknown>(tool: string, settleAfter = 0) {
  return useMutation({ mutationKey: [tool], mutationFn: async (args: A) => (await Promise.all([call<T>(tool, args), new Promise((done) => setTimeout(done, settleAfter))]))[0] })
}

export function useObjects() {
  return useTool<{ objects: CrmObject[] }>('list_objects', {}, { staleTime: 60_000 }).data?.objects
}

export function useWorkspace() {
  return useTool<Workspace>('get_workspace').data
}

// useRecords finds records to pick once a picker opens and searches, its
// query undefined until then.
export function useRecords(object: string, query: string | undefined, limit: number) {
  return useTool<{ records: CrmRecord[] }>('search_records', { object, query, limit }, { enabled: query !== undefined, placeholderData: (previous) => previous })
}

export function useWrite(record: CrmRecord) {
  const upsert = useAction<{ object: string; record_id: string; values?: Record<string, string>; remove?: Record<string, string[]> }>('upsert_record')
  return {
    pending: upsert.isPending,
    set: (slug: string, value: string) => upsert.mutate({ object: record.object, record_id: record.id, values: { [slug]: value } }),
    remove: (slug: string, values: string[]) => upsert.mutate({ object: record.object, record_id: record.id, remove: { [slug]: values } }),
  }
}

// useRecordPages pages through an object's records, each page with the count
// of every match; pages repeat no record when others were added meanwhile.
export function useRecordPages(object: string, query: string, filters: RecordFilter[], include: Relation[] | undefined, sort: string | undefined, limit: number) {
  const args = { object, query, filters, include, sort, limit }
  const result = useInfiniteQuery({
    queryKey: ['search_records', 'pages', args],
    queryFn: ({ pageParam }) => call<{ records: CrmRecord[]; total: number }>('search_records', { ...args, offset: pageParam }),
    initialPageParam: 0,
    getNextPageParam: (last, pages) => {
      const loaded = pages.reduce((sum, page) => sum + page.records.length, 0)
      return last.records.length > 0 && loaded < last.total ? loaded : undefined
    },
    placeholderData: (previous) => previous,
  })
  const pages = result.data?.pages
  const records = useMemo(() => pages && [...new Map(pages.flatMap((page) => page.records).map((r) => [r.id, r])).values()], [pages])
  return { ...result, records, total: pages?.[0]?.total }
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

// useUpcoming lists a record's scheduled interactions, soonest first, for
// records that have them.
export function useUpcoming(recordId: string, enabled = true) {
  return useTool<{ interactions: Interaction[] }>('list_interactions', { record_id: recordId, upcoming: true, limit: 5 }, { enabled }).data?.interactions ?? []
}

// useTimeline pages back through a record's interactions of some kinds, or
// of every kind.
export function useTimeline(recordId: string, kinds?: string[]) {
  return useInfiniteQuery({
    queryKey: ['list_interactions', recordId, kinds],
    queryFn: ({ pageParam }) => call<{ interactions: Interaction[] }>('list_interactions', { record_id: recordId, kinds, cursor: pageParam, limit: pageSize }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => (last.interactions.length === pageSize ? last.interactions[pageSize - 1].id : undefined),
  })
}
