import { useState } from 'react'
import { useAction, useTool } from '@/lib/queries'
import type { TriageRule } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Button } from '@jaz/ui/button'
import { Row, inputClass } from './controls'

export function TriageDomainRules() {
  const rules = useTool<{ rules: TriageRule[] }>('list_triage_rules').data?.rules ?? []
  const decide = useAction<{ domains: string[]; decision: string }>('decide_triage')
  const forget = useAction<{ domain: string }>('forget_triage_rule')
  const [domain, setDomain] = useState('')
  const [decision, setDecision] = useState('skip')
  return (
    <>
      <Row><div><p className="font-medium text-ink">Domain rules</p><p className="text-[12px] text-ink-3">Apply to current and future addresses. Your address-specific decisions remain after removing a rule.</p></div></Row>
      {rules.map((rule) => (
        <Row key={rule.domain}>
          <div className="min-w-0 flex-1"><p className="truncate text-ink">{rule.domain}</p>{rule.reason && <p className="truncate text-[12px] text-ink-3">{rule.reason}</p>}</div>
          <span className="text-ink-2">{rule.decision === 'kept' ? 'Keep' : 'Skip'}</span>
          <Button disabled={forget.isPending} onClick={() => forget.mutate({ domain: rule.domain })} aria-label={`Remove rule for ${rule.domain}`}>Remove</Button>
        </Row>
      ))}
      <Row>
        <form className="flex min-w-0 flex-1 flex-wrap gap-2" onSubmit={(event) => {
          event.preventDefault()
          decide.mutate({ domains: [domain], decision }, { onSuccess: () => setDomain('') })
        }}>
          <input aria-label="Email domain" value={domain} onChange={(event) => setDomain(event.target.value)} placeholder="company.com" className={cn(inputClass, 'min-w-32 flex-1')} />
          <select aria-label="Domain decision" value={decision} onChange={(event) => setDecision(event.target.value)} className={inputClass}><option value="skip">Always skip</option><option value="keep">Always keep</option></select>
          <Button type="submit" disabled={!domain.trim() || decide.isPending}>Add rule</Button>
        </form>
      </Row>
    </>
  )
}
