import { Button } from '@jaz/ui/button'
import { ArrowDownLeft, ArrowUpRight, ChevronDown } from 'lucide-react'
import { useState } from 'react'
import { formatDate, formatDateTime, formatTime, timeAgo } from '@/lib/format'
import type { Interaction, CrmMessage } from '@/lib/types'
import { cn } from '@/lib/utils'
import { RecordIcon } from './icons'
import { Message } from './message'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

export function MessageState({ message }: { message: CrmMessage }) {
  const author = message.sender || message.sender_address
  return (
    <span className="inline-flex min-w-0 max-w-full items-center gap-1.5" title={message.sender_address}>
      {message.direction === 'sent' && <ArrowUpRight className="size-3.5 shrink-0" />}
      {message.direction === 'received' && <ArrowDownLeft className="size-3.5 shrink-0" />}
      <span className="truncate">{message.direction === 'sent' ? 'Last sent by' : message.direction === 'received' ? 'Latest from' : 'Last message from'} {author || 'unknown sender'}</span>
      <span className="shrink-0 text-ink-3">· {timeAgo(message.at)}</span>
    </span>
  )
}

const authorOf = (message: CrmMessage) => message.sender || message.sender_address || 'Unknown sender'

// runs groups consecutive messages from one sender on one day, as a chat
// shows them.
function runs(messages: CrmMessage[]) {
  const out: { day: string; author: string; messages: CrmMessage[] }[] = []
  for (const message of messages) {
    const day = formatDate(message.at)
    const author = authorOf(message)
    const run = out.at(-1)
    if (run?.day === day && run.author === author) {
      run.messages.push(message)
    } else {
      out.push({ day, author, messages: [message] })
    }
  }
  return out
}

export function MessageThread({ interaction, messages, initialVisible = 6 }: { interaction: Interaction; messages: CrmMessage[]; initialVisible?: number }) {
  const [expanded, setExpanded] = useState(false)
  const start = expanded ? 0 : Math.max(0, messages.length - initialVisible)
  const participant = (address?: string) => interaction.participants.find((p) => p.address === address)
  return (
    <section aria-label="Messages" className="flex min-w-0 flex-col gap-3">
      {messages.length > initialVisible && (
        <Button onClick={() => setExpanded(!expanded)} variant="ghost" size="sm" aria-expanded={expanded} className="self-center">
          {expanded ? 'Hide earlier messages' : <>Show thread<span className="tabular-nums text-ink-3">· {messages.length - initialVisible} earlier</span></>}
          <ChevronDown aria-hidden="true" className={cn('transition-transform duration-150 motion-reduce:transition-none', expanded && 'rotate-180')} />
        </Button>
      )}
      <ol className="flex min-w-0 flex-col gap-4">
        {runs(messages.slice(start)).map((run, i, all) => {
          const latest = run.messages[run.messages.length - 1]
          const sent = latest.direction === 'sent'
          const recipients = [...new Set(latest.recipients?.map((address) => participant(address)?.name || address))].filter((name) => name !== run.author)
          return (
            <li key={i} className="flex min-w-0 flex-col gap-4">
              {run.day !== all[i - 1]?.day && <time dateTime={run.messages[0].at} className="self-center text-[11.5px] text-ink-3">{run.day}</time>}
              <div className={cn('flex max-w-[88%] items-end gap-2 sm:max-w-[76%]', sent ? 'flex-row-reverse self-end' : 'self-start')}>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="shrink-0">
                      <RecordIcon object="people" name={run.author} photo={participant(latest.sender_address)?.photo} size={24} />
                    </span>
                  </TooltipTrigger>
                  <TooltipContent side="top" align={sent ? 'end' : 'start'} className="flex flex-col">
                    <span className="font-medium">{run.author}</span>
                    {latest.sender_address !== run.author && <span>{latest.sender_address}</span>}
                    {recipients.length > 0 && <span>to {recipients.join(', ')}</span>}
                  </TooltipContent>
                </Tooltip>
                <div className={cn('flex min-w-0 flex-col gap-1', sent ? 'items-end' : 'items-start')}>
                  {run.messages.map((message, j) => (
                    <div key={j} className={cn('min-w-0 max-w-full rounded-[18px] px-3.5 pb-1.5 pt-2', sent ? 'bg-primary-soft last:rounded-br-[6px]' : 'bg-list-hover last:rounded-bl-[6px]')}>
                      <span className="sr-only">{run.author}: </span>
                      {message.text || message.html ? <Message text={message.text} html={message.html} /> : <p className="text-[13px] text-ink-3">The text arrives with the next sync.</p>}
                      {message.partial && <p className="mt-2 text-[11.5px] text-ink-3">Message excerpt</p>}
                      {message.at.length > 10 && <time dateTime={message.at} title={formatDateTime(message.at)} className="mt-0.5 block text-right text-[11px] tabular-nums text-ink-3">{formatTime(message.at)}</time>}
                    </div>
                  ))}
                </div>
              </div>
            </li>
          )
        })}
      </ol>
    </section>
  )
}
