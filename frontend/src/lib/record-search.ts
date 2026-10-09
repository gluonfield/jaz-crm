import { z } from 'zod'
import { defaultParseSearch } from '@tanstack/react-router'
import { filterOperators, type RecordFilter } from './types'

export type RecordSearchScope = { group_by_conversation?: boolean; conversation_id?: string }

// Search keys the record list itself uses. Every other key is a filter:
// attribute=value, or attribute.operator=value for any other operator.
const listKeys = new Set(['q', 'sort', 'view', 'limit', 'saved', 'filters', 'where', 'category', 'group_by_conversation', 'conversation_id'])

export const needsValue = (filter: RecordFilter) => !['is_empty', 'is_not_empty'].includes(filter.operator)

const isOperator = (value: string): value is RecordFilter['operator'] => (filterOperators as readonly string[]).includes(value)

// keyFilters reads the filters written as their own search keys, and the
// keys it read.
function keyFilters(search: Record<string, unknown>) {
  const filters: RecordFilter[] = []
  const keys: string[] = []
  for (const [key, raw] of Object.entries(search)) {
    const [attribute, operator = 'is', ...extra] = key.split('.')
    if (listKeys.has(key) || extra.length || !isOperator(operator)) {
      continue
    }
    keys.push(key)
    for (const value of Array.isArray(raw) ? raw : [raw]) {
      const filter: RecordFilter = { attribute, operator }
      if (!['string', 'number', 'boolean'].includes(typeof value)) {
        continue
      }
      if (!needsValue(filter)) {
        filters.push(filter)
      } else if (String(value)) {
        filters.push({ ...filter, value: String(value) })
      }
    }
  }
  return { filters, keys }
}

// filterKeys writes filters as their own search keys. A filter those keys
// cannot express, such as a second value for one key or an attribute named
// like a list key, stays in filters; an empty list means no filters at all,
// rather than the list's defaults.
export function filterKeys(filters: RecordFilter[] | undefined): Record<string, unknown> {
  if (!filters?.length) {
    return { filters }
  }
  const keys: Record<string, string> = {}
  const rest: RecordFilter[] = []
  for (const filter of filters) {
    const key = filter.operator === 'is' ? filter.attribute : `${filter.attribute}.${filter.operator}`
    if (listKeys.has(filter.attribute) || filter.attribute.includes('.') || key in keys) {
      rest.push(filter)
    } else {
      keys[key] = filter.value ?? ''
    }
  }
  return { ...keys, filters: rest.length ? rest : undefined }
}

export function validateRecordSearch(search: Record<string, unknown>): RecordSearchScope & { sort?: 'name' | 'updated_at'; view?: 'table' | 'board'; q?: string; filters?: RecordFilter[]; saved?: string; limit?: number; where?: never; category?: never } {
  const filters = z.array(z.object({ attribute: z.string(), operator: z.enum(filterOperators), value: z.string().optional() })).parse(search.filters ?? [])
  if (search.where && typeof search.where === 'object' && !Array.isArray(search.where)) {
    for (const [attribute, value] of Object.entries(search.where)) {
      if (typeof value === 'string') {
        filters.push({ attribute, operator: 'is', value })
      }
    }
  }
  if (typeof search.category === 'string' && search.category) {
    filters.push({ attribute: 'categories', operator: 'is', value: search.category })
  }
  const read = keyFilters(search)
  filters.push(...read.filters)
  // The router keeps the URL's keys beside the validated search, so keys read
  // into filters are cleared; filters alone then say which conditions apply.
  return {
    ...Object.fromEntries(read.keys.map((key) => [key, undefined])),
    where: undefined,
    category: undefined,
    view: search.view === 'table' || search.view === 'board' ? search.view : undefined,
    sort: search.sort === 'name' || search.sort === 'updated_at' ? search.sort : undefined,
    q: ['string', 'number', 'boolean'].includes(typeof search.q) ? String(search.q) : undefined,
    filters: filters.length || Array.isArray(search.filters) ? filters : undefined,
    saved: typeof search.saved === 'string' ? search.saved : undefined,
    limit: typeof search.limit === 'number' && Number.isInteger(search.limit) ? Math.max(1, Math.min(search.limit, 100)) : undefined,
    group_by_conversation: typeof search.group_by_conversation === 'boolean' ? search.group_by_conversation : undefined,
    conversation_id: typeof search.conversation_id === 'string' ? search.conversation_id : undefined,
  }
}

export function recordSearchInput(path: string) {
  const location = new URL(path, 'http://crm')
  const object = /^\/o\/([^/]+)$/.exec(location.pathname)?.[1] ?? ''
  const { q = '', filters = [], limit = 100, group_by_conversation, conversation_id, sort } = validateRecordSearch(defaultParseSearch(location.search))
  return { object: decodeURIComponent(object), query: q.trim(), limit, filters, group_by_conversation, conversation_id, ...(sort ? { sort } : {}) }
}
