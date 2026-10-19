import { createRoot } from 'react-dom/client'
import { createMemoryHistory, createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router'
import { QueryClientProvider } from '@tanstack/react-query'
import { RecordTable } from '../src/components/record-table'
import { newQueryClient, useObjects, useTool } from '../src/lib/queries'
import type { CrmRecord } from '../src/lib/types'

const slug = new URLSearchParams(location.search).get('object') ?? 'events'
function Table() {
  const objects = useObjects()
  const object = objects?.find((o) => o.slug === slug)
  const records = useTool<{ records: CrmRecord[] }>('search_records', { object: slug }).data?.records
  return object && records && <div style={{ overflow: 'auto', height: 400 }}><RecordTable object={object} objects={objects!} records={records} focus={-1} openRecord={() => {}} /></div>
}
const router = createRouter({ routeTree: createRootRoute({ component: Table }), history: createMemoryHistory() })
const client = newQueryClient()
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>)
