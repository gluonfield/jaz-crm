import { formatActionDate } from '@/lib/dates'
import { useTool } from '@/lib/queries'
import type { OperationError } from '@/lib/types'
import { Row, Section } from './controls'

export function OperationErrors({ timezone }: { timezone: string }) {
  const { data, error, isPending } = useTool<{ errors: OperationError[] }>('get_operation_errors')
  if (!isPending && !error && !data?.errors.length) {
    return null
  }
  return (
    <Section title="Needs attention">
      {isPending && <Row><span role="status" className="text-ink-2">Checking for reported issues…</span></Row>}
      {error && <Row><span role="status" className="text-danger">{data ? 'Reported issues could not be refreshed.' : 'Could not load reported issues.'}</span></Row>}
      {data?.errors.map((issue) => (
        <Row key={issue.id}>
          <div className="min-w-0 flex-1 break-words">
            <div className="font-medium text-ink">{issue.operation}</div>
            <p className="mt-0.5 text-ink-2">{issue.reason}</p>
            <div className="mt-1.5 flex flex-wrap gap-x-4 gap-y-1 text-[12px] text-ink-3">
              <span>Last occurred <time dateTime={issue.occurred_at}>{formatActionDate(issue.occurred_at, timezone)}</time></span>
              {issue.retry_at && <span>Next check <time dateTime={issue.retry_at}>{formatActionDate(issue.retry_at, timezone)}</time></span>}
            </div>
          </div>
        </Row>
      ))}
    </Section>
  )
}
