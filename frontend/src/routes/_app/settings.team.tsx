import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { Button } from '@jaz/ui/button'
import { Row, Section, inputClass } from '@/components/controls'
import { useAction, useWorkspace } from '@/lib/queries'
import { cn } from '@/lib/utils'
import type { Member } from '@/lib/types'

export const Route = createFileRoute('/_app/settings/team')({ component: TeamPage })

function TeamPage() {
  const workspace = useWorkspace()
  const admin = workspace?.members?.find((m) => m.is_me)?.admin
  return (
    <Section id="members" title="Team">
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
  )
}

function OtherAddresses({ member, editable }: { member: Member; editable: boolean }) {
  const current = member.addresses?.join(', ') ?? ''
  const [text, setText] = useState(current)
  const update = useAction<object>('update_member')
  if (!editable) {
    return current ? <div className="truncate text-[12px] text-ink-3">Also sends from {current}</div> : null
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
