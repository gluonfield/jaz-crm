import assert from 'node:assert/strict'
import { test } from 'node:test'
import { QueryClientProvider } from '@tanstack/react-query'
import { renderToStaticMarkup } from 'react-dom/server'
import { setTransport } from '@/lib/api'
import { newQueryClient, toolQuery } from '@/lib/queries'
import { OperationErrors } from './operation-errors'

test('reported operation errors show safe details and workspace times', () => {
  const client = newQueryClient()
  client.setQueryData(['get_operation_errors', {}], { errors: [
    { id: 'private-issue-id', operation: 'Drafting paused', reason: 'Provider billing needs attention.', occurred_at: '2026-11-12T20:00:00Z', retry_at: '2026-11-12T20:30:00Z' },
    { id: 'second-private-id', operation: 'Contact triage paused', reason: 'Provider quota has been reached.', occurred_at: '2026-11-12T20:15:00Z' },
  ] })
  const html = renderToStaticMarkup(<QueryClientProvider client={client}><OperationErrors timezone="Asia/Kolkata" /></QueryClientProvider>)
  assert.match(html, /Needs attention/)
  assert.match(html, /Drafting paused/)
  assert.match(html, /Provider billing needs attention\./)
  assert.match(html, /Contact triage paused/)
  assert.match(html, /Provider quota has been reached\./)
  assert.match(html, /datetime="2026-11-12T20:00:00Z">Nov 13(?:, 2026)?, 01:30<\/time>/i)
  assert.match(html, /datetime="2026-11-12T20:30:00Z">Nov 13(?:, 2026)?, 02:00<\/time>/i)
  assert.equal(html.match(/Next check/g)?.length, 1)
  assert.doesNotMatch(html, /private-issue-id|second-private-id/)
  client.clear()
})

test('an unknown report shows loading while an empty report makes no health claim', () => {
  const client = newQueryClient()
  const loading = renderToStaticMarkup(<QueryClientProvider client={client}><OperationErrors timezone="UTC" /></QueryClientProvider>)
  assert.match(loading, /role="status"[^>]*>Checking for reported issues…/)
  client.setQueryData(['get_operation_errors', {}], { errors: [] })
  assert.equal(renderToStaticMarkup(<QueryClientProvider client={client}><OperationErrors timezone="UTC" /></QueryClientProvider>), '')
  client.clear()
})

test('a failed report stays explicit and retains previously reported issues', async () => {
  const client = newQueryClient()
  client.setDefaultOptions({ queries: { retryOnMount: false } })
  setTransport(async () => {
    throw new Error('Sensitive raw provider response')
  })
  try {
    const query = { ...toolQuery('get_operation_errors', {}), retry: false, staleTime: 0 }
    await assert.rejects(client.fetchQuery(query), /Sensitive raw provider response/)
    const failed = renderToStaticMarkup(<QueryClientProvider client={client}><OperationErrors timezone="UTC" /></QueryClientProvider>)
    assert.match(failed, /Could not load reported issues\./)
    assert.doesNotMatch(failed, /Checking|Sensitive raw provider response/)
    client.setQueryData(query.queryKey, { errors: [{ id: 'issue', operation: 'Drafting paused', reason: 'Provider quota has been reached.', occurred_at: '2026-11-12T20:00:00Z' }] })
    await assert.rejects(client.fetchQuery(query), /Sensitive raw provider response/)
    const stale = renderToStaticMarkup(<QueryClientProvider client={client}><OperationErrors timezone="UTC" /></QueryClientProvider>)
    assert.match(stale, /Reported issues could not be refreshed\./)
    assert.match(stale, /Drafting paused/)
    assert.match(stale, /Provider quota has been reached\./)
    assert.doesNotMatch(stale, /Sensitive raw provider response/)
  } finally {
    client.clear()
    setTransport(null)
  }
})
