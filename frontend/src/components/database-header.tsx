import { Button } from '@jaz/ui/button'
import { Table2 } from 'lucide-react'
import { useAction } from '@/lib/queries'
import type { CrmObject } from '@/lib/types'
import { IconPicker, PageIcon } from './page-icon'

export function DatabaseHeader({ object }: { object: CrmObject }) {
  const edit = useAction<object>('edit_object')
  return <section className="shrink-0 px-5 pb-5 pt-8 md:px-8">
    <div className="flex items-start gap-3">
      <IconPicker value={object.icon} pending={edit.isPending} onChange={(icon, onSuccess) => edit.mutate({ object: object.slug, action: 'icon', icon: icon ?? '' }, { onSuccess })}>
        <Button variant="ghost" aria-label="Change database icon" className="-ml-1 size-11 shrink-0">
          <PageIcon value={object.icon} size={32} fallback={Table2} />
        </Button>
      </IconPicker>
      <h1 className="min-w-0 flex-1 text-[30px] font-semibold leading-[1.35] tracking-[-0.02em] text-ink">
        <textarea key={object.name} aria-label="Database name" rows={1} defaultValue={object.name} onBlur={(event) => {
          const input = event.currentTarget
          const entered = input.value
          const next = entered.replace(/\n/g, ' ').trim()
          if (next && next !== object.name) {
            edit.mutate({ object: object.slug, action: 'rename', name: next }, { onError: () => {
              if (input.value === entered) {
                input.value = object.name
              }
            } })
          } else {
            input.value = object.name
          }
        }} onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === 'Escape') {
            event.preventDefault()
            if (event.key === 'Escape') {
              event.currentTarget.value = object.name
            }
            event.currentTarget.blur()
          }
        }} className="field-sizing-content block w-full resize-none bg-transparent py-0.5 outline-none" />
      </h1>
    </div>
    <textarea key={object.description ?? ''} aria-label="Database description" rows={1} defaultValue={object.description ?? ''} placeholder="Add description…" onBlur={(event) => {
      const input = event.currentTarget
      const entered = input.value
      const next = entered.trim()
      if (next !== (object.description ?? '')) {
        edit.mutate({ object: object.slug, action: 'describe', description: next }, { onError: () => {
          if (input.value === entered) {
            input.value = object.description ?? ''
          }
        } })
      }
    }} className="field-sizing-content mt-2 block w-full resize-none bg-transparent text-[13px] leading-relaxed text-ink-2 outline-none placeholder:text-ink-3" />
  </section>
}
