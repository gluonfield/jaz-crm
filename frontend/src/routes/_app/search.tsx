import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { TextSearch } from 'lucide-react'
import { Header, inputClass } from '@/components/controls'
import { EmptyState } from '@/components/empty-state'
import { InteractionRow } from '@/components/timeline'
import { useTool } from '@/lib/queries'
import type { Interaction } from '@/lib/types'
import { cn } from '@/lib/utils'
import { useDebounced } from '@/lib/hooks'
import { useState } from 'react'

export const Route = createFileRoute('/_app/search')({
  validateSearch: (search: Record<string, unknown>) => ({ q: typeof search.q === 'string' ? search.q : '' }),
  component: SearchPage,
})

function SearchPage() {
  const { q } = Route.useSearch()
  const navigate = useNavigate()
  const [text, setText] = useState(q)
  const query = useDebounced(text.trim(), 300)
  const found = useTool<{ interactions: Interaction[] }>('search_interactions', { query, limit: 50 }, { enabled: !!query }).data?.interactions
  return (
    <>
      <Header>
        <TextSearch />
        Conversations
        <input
          autoFocus
          value={text}
          onChange={(e) => {
            setText(e.target.value)
            void navigate({ to: '/search', search: { q: e.target.value }, replace: true })
          }}
          placeholder="Search emails, meetings, calls and notes"
          className={cn(inputClass, 'ml-3 w-96')}
        />
      </Header>
      {query && found?.length === 0 ? (
        <EmptyState title="No conversations match" icon={<TextSearch />} />
      ) : (
        <ol className="scrollbar-quiet mx-auto w-full max-w-[820px] min-h-0 flex-1 overflow-y-auto px-7 py-3">
          {found?.map((i) => (
            <InteractionRow key={i.id} interaction={i} />
          ))}
        </ol>
      )}
    </>
  )
}
