import { createFileRoute } from '@tanstack/react-router'
import { Plus, Settings } from 'lucide-react'
import { useState } from 'react'
import { Button, Header, Row, Section, inputClass } from '@/components/controls'
import { useAPIKeys, useCreateAPIKey, useGrants, useRevoke } from '@/lib/account'
import { embedded } from '@/lib/api'
import { slugify } from '@/lib/crm'
import { formatDate } from '@/lib/format'
import { useAction, useObjects, useWorkspace } from '@/lib/queries'
import { type AttributeType, type CrmObject, attributeTypes } from '@/lib/types'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/_app/settings')({ component: SettingsPage })

function SettingsPage() {
  const workspace = useWorkspace()
  const objects = useObjects() ?? []
  const admin = workspace?.members?.find((m) => m.is_me)?.admin
  return (
    <>
      <Header>
        <Settings />
        Settings
      </Header>
      <div className="scrollbar-quiet min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-[680px] px-10 pb-16 pt-9">
          {workspace && <WorkspaceSection key={workspace.name + workspace.description} name={workspace.name} description={workspace.description} admin={!!admin} />}
          <Section id="members" title="Members">
            {workspace?.members?.map((m) => (
              <Row key={m.email}>
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium text-ink">{m.name}</div>
                  <div className="truncate text-[12px] text-ink-3">{m.email}</div>
                </div>
                {m.admin && <span className="text-[12px] text-ink-3">Admin</span>}
              </Row>
            ))}
            {workspace?.invited?.map((email) => (
              <Row key={email} className="text-ink-3">
                <span className="flex-1 truncate">{email}</span>
                <span className="text-[12px]">Invited</span>
              </Row>
            ))}
            {admin && <Invite />}
          </Section>
          <Section title="Objects">
            {objects.map((o) => (
              <ObjectRow key={o.slug} object={o} objects={objects} />
            ))}
            <NewObject />
          </Section>
          {!embedded() && <Credentials />}
        </div>
      </div>
    </>
  )
}

function WorkspaceSection({ name, description, admin }: { name: string; description: string; admin: boolean }) {
  const [draft, setDraft] = useState({ name, description })
  const update = useAction<object>('update_workspace')
  const changed = draft.name !== name || draft.description !== description
  return (
    <Section title="Workspace">
      <Row>
        <span className="w-28 shrink-0 text-ink-2">Name</span>
        <input aria-label="Workspace name" value={draft.name} disabled={!admin} onChange={(e) => setDraft({ ...draft, name: e.target.value })} className={cn(inputClass, 'flex-1')} />
      </Row>
      <Row className="items-start">
        <span className="mt-1 w-28 shrink-0 text-ink-2">Who belongs</span>
        <textarea
          aria-label="Who belongs"
          value={draft.description}
          disabled={!admin}
          rows={3}
          onChange={(e) => setDraft({ ...draft, description: e.target.value })}
          placeholder="Customers, suppliers and partners of our press shop"
          className={cn(inputClass, 'h-auto flex-1 resize-none py-1.5 leading-[1.5]')}
        />
      </Row>
      {admin && changed && (
        <Row className="justify-end">
          <Button primary onClick={() => update.mutate(draft)}>
            Save
          </Button>
        </Row>
      )}
    </Section>
  )
}

function Invite() {
  const [email, setEmail] = useState('')
  const invite = useAction<object>('invite_member')
  return (
    <Row>
      <form
        className="flex flex-1 gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          invite.mutate({ email }, { onSuccess: () => setEmail('') })
        }}
      >
        <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="name@company.com" className={cn(inputClass, 'flex-1')} />
        <Button type="submit" disabled={!email}>
          Invite
        </Button>
      </form>
    </Row>
  )
}

function ObjectRow({ object, objects }: { object: CrmObject; objects: CrmObject[] }) {
  const [adding, setAdding] = useState(false)
  return (
    <div className="border-b border-border/70 px-4 py-3 last:border-b-0">
      <div className="flex items-center gap-2 text-[13px]">
        <span className="font-medium text-ink">{object.name}</span>
        <span className="text-ink-3">{object.slug}</span>
        <Button className="ml-auto border-transparent" aria-label={`Add attribute to ${object.name}`} onClick={() => setAdding(!adding)}>
          <Plus /> Attribute
        </Button>
      </div>
      <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[12.5px] text-ink-2">
        {object.attributes.map((a) => (
          <span key={a.slug}>
            {a.name} <span className="text-ink-3">{[a.type, a.target, a.multi && 'many', a.unique && 'unique'].filter(Boolean).join(' · ')}</span>
          </span>
        ))}
      </div>
      {adding && <NewAttribute object={object} objects={objects} onDone={() => setAdding(false)} />}
    </div>
  )
}

function NewAttribute({ object, objects, onDone }: { object: CrmObject; objects: CrmObject[]; onDone: () => void }) {
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
      className="mt-3 flex flex-wrap items-center gap-2"
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
      <Button primary type="submit" disabled={!slugify(name)}>
        Add
      </Button>
    </form>
  )
}

function NewObject() {
  const [name, setName] = useState('')
  const create = useAction<object>('create_object')
  return (
    <Row>
      <form
        className="flex flex-1 gap-2"
        onSubmit={(e) => {
          e.preventDefault()
          create.mutate({ slug: slugify(name), name }, { onSuccess: () => setName('') })
        }}
      >
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="New object, such as Deals" className={cn(inputClass, 'flex-1')} />
        <Button type="submit" disabled={!slugify(name)}>
          Create
        </Button>
      </form>
    </Row>
  )
}

function Credentials() {
  const keys = useAPIKeys()
  const grants = useGrants()
  const createKey = useCreateAPIKey()
  const revoke = useRevoke()
  const [label, setLabel] = useState('')
  return (
    <>
      <Section title="API keys">
        {createKey.data && keys.some((k) => k.id === createKey.data.apiKey.id) && (
          <Row>
            <code className="flex-1 select-all break-all font-mono text-[12px] text-ink">{createKey.data.key}</code>
            <span className="text-[12px] text-ink-3">Shown once</span>
          </Row>
        )}
        {keys.map((k) => (
          <Row key={k.id}>
            <span className="flex-1 truncate text-ink">{k.label}</span>
            <span className="font-mono text-[12px] text-ink-3">{k.hint}</span>
            <span className="text-[12px] text-ink-3">{formatDate(k.createdAt)}</span>
            <Button onClick={() => revoke.mutate(`/auth/api-keys/${k.id}`)}>Revoke</Button>
          </Row>
        ))}
        <Row>
          <form
            className="flex flex-1 gap-2"
            onSubmit={(e) => {
              e.preventDefault()
              createKey.mutate(label, { onSuccess: () => setLabel('') })
            }}
          >
            <input value={label} onChange={(e) => setLabel(e.target.value)} placeholder="Label, such as Jaz" className={cn(inputClass, 'flex-1')} />
            <Button type="submit" disabled={!label.trim()}>
              Create key
            </Button>
          </form>
        </Row>
      </Section>
      {grants.length > 0 && (
        <Section title="Authorized apps">
          {grants.map((g) => (
            <Row key={g.id}>
              <span className="flex-1 truncate text-ink">{g.clientName}</span>
              <span className="text-[12px] text-ink-3">{formatDate(g.lastUsedAt)}</span>
              <Button onClick={() => revoke.mutate(`/auth/grants/${g.id}`)}>Revoke</Button>
            </Row>
          ))}
        </Section>
      )}
    </>
  )
}
