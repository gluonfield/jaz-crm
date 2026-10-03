import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { Button } from '@jaz/ui/button'
import { Row, Section, inputClass } from '@/components/controls'
import { cn } from '@/lib/utils'
import { McpConnection } from '@/components/mcp-connection'
import { useAPIKeys, useCreateAPIKey, useGrants, useRevoke } from '@/lib/account'
import { embedded } from '@/lib/api'
import { formatDate } from '@/lib/format'

export const Route = createFileRoute('/_app/settings/access')({ component: AccessPage })

function AccessPage() {
  return (
    <>
      <Section id="mcp" title="MCP"><McpConnection /></Section>
      {!embedded() && <Credentials />}
    </>
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
