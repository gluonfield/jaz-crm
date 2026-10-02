import { createFileRoute } from '@tanstack/react-router'
import { Plus, Settings } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@jaz/ui/button'
import { Header, Row, Section, inputClass } from '@/components/controls'
import { DeleteWorkspace } from '@/components/delete-workspace'
import { McpConnection } from '@/components/mcp-connection'
import { NewAttribute } from '@/components/new-attribute'
import { TriageSettings } from '@/components/triage-settings'
import { useAPIKeys, useCreateAPIKey, useGrants, useRevoke } from '@/lib/account'
import { embedded } from '@/lib/api'
import { slugify } from '@/lib/crm'
import { formatDate } from '@/lib/format'
import { useAction, useObjects, useWorkspace } from '@/lib/queries'
import type { CrmObject, Member } from '@/lib/types'
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
                  <div className="flex items-baseline gap-2">
                    <span className="truncate font-medium text-ink">{m.name}</span>
                    {m.admin && <span className="ml-auto text-[12px] text-ink-3">Admin</span>}
                  </div>
                  <div className="truncate text-[12px] text-ink-3">{m.email}</div>
                  <OtherAddresses key={m.addresses?.join()} member={m} editable={!!(m.is_me || admin)} />
                </div>
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
          <TriageSettings admin={!!admin} />
          <Section title="Objects">
            {objects.map((o) => (
              <ObjectRow key={o.slug} object={o} objects={objects} />
            ))}
            <NewObject />
          </Section>
          <Section id="mcp" title="MCP"><McpConnection /></Section>
          {!embedded() && <Credentials />}
          {workspace && admin && <DeleteWorkspace key={workspace.id + workspace.name} workspace={workspace} />}
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
          <Button variant="primary" onClick={() => update.mutate(draft)}>
            Save
          </Button>
        </Row>
      )}
    </Section>
  )
}

// OtherAddresses lists the other addresses a member sends from, so their mail
// from those reads as ours.
function OtherAddresses({ member, editable }: { member: Member; editable: boolean }) {
  const current = member.addresses?.join(', ') ?? ''
  const [text, setText] = useState(current)
  const update = useAction<object>('update_member')
  if (!editable) {
    return current && <div className="truncate text-[12px] text-ink-3">Also sends from {current}</div>
  }
  return (
    <input
      aria-label={`Other addresses ${member.name} sends from`}
      value={text}
      disabled={update.isPending}
      placeholder={member.is_me ? 'Other addresses you send from' : `Other addresses ${member.name} sends from`}
      onChange={(e) => setText(e.target.value)}
      onBlur={() => text !== current && update.mutate({ email: member.email, addresses: text.split(/[\s,;]+/).filter(Boolean) }, { onError: () => setText(current) })}
      onKeyDown={(e) => {
        if (e.key === 'Enter') {
          e.currentTarget.blur()
        } else if (e.key === 'Escape') {
          setText(current)
        }
      }}
      className={cn(inputClass, 'mt-2 w-full text-[12px]')}
    />
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
        <Button variant="ghost" className="ml-auto" aria-label={`Add attribute to ${object.name}`} onClick={() => setAdding(!adding)}>
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
      {adding && <NewAttribute object={object} objects={objects} onDone={() => setAdding(false)} className="mt-3" />}
    </div>
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
