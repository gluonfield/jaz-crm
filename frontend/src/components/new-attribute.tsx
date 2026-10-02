import { Button } from '@jaz/ui/button'
import { useState } from 'react'
import { slugify } from '@/lib/crm'
import { useAction } from '@/lib/queries'
import { type AttributeType, type CrmObject, attributeTypes } from '@/lib/types'
import { cn } from '@/lib/utils'
import { inputClass } from './controls'

// NewAttribute adds an attribute to an object, such as a column to a table.
export function NewAttribute({ object, objects, onDone, className }: { object: CrmObject; objects: CrmObject[]; onDone: () => void; className?: string }) {
  const [name, setName] = useState('')
  const [type, setType] = useState<AttributeType>('text')
  const [target, setTarget] = useState(objects[0]?.slug ?? '')
  const [options, setOptions] = useState('')
  const [multi, setMulti] = useState(false)
  const create = useAction<object>('create_attribute')
  const submit = () =>
    create.mutate(
      {
        object: object.slug,
        slug: slugify(name),
        name,
        type,
        multi,
        target: type === 'reference' ? target : undefined,
        options: type === 'select' || type === 'status' ? options.split(',').map((o) => o.trim()).filter(Boolean) : undefined,
      },
      { onSuccess: onDone },
    )
  return (
    <form
      className={cn('flex flex-wrap items-center gap-2', className)}
      onSubmit={(e) => {
        e.preventDefault()
        submit()
      }}
    >
      <input autoFocus value={name} onChange={(e) => setName(e.target.value)} placeholder="Name" className={cn(inputClass, 'w-40')} />
      <select aria-label="Type" value={type} onChange={(e) => setType(e.target.value as AttributeType)} className={cn(inputClass, 'w-28')}>
        {attributeTypes.map((t) => (
          <option key={t}>{t}</option>
        ))}
      </select>
      {type === 'reference' && (
        <select aria-label="Links to" value={target} onChange={(e) => setTarget(e.target.value)} className={cn(inputClass, 'w-32')}>
          {objects.map((o) => (
            <option key={o.slug} value={o.slug}>
              {o.name}
            </option>
          ))}
        </select>
      )}
      {(type === 'select' || type === 'status') && <input value={options} onChange={(e) => setOptions(e.target.value)} placeholder="Lead, Quoted, Won" className={cn(inputClass, 'w-48')} />}
      {type !== 'checkbox' && (
        <label className="flex items-center gap-1.5 text-[12.5px] text-ink-2">
          <input type="checkbox" checked={multi} onChange={(e) => setMulti(e.target.checked)} /> Many
        </label>
      )}
      <Button variant="primary" type="submit" disabled={!slugify(name)}>
        Add
      </Button>
    </form>
  )
}
