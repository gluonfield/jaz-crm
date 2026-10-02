import { Button } from '@jaz/ui/button'
import { ArrowDownLeft, ArrowUpRight, ChevronDown } from 'lucide-react'
import { useState } from 'react'
import { formatDate, formatDateTime, formatTime, timeAgo } from '@/lib/format'
import type { Interaction, CrmMessage } from '@/lib/types'
import { cn } from '@/lib/utils'
import { RecordIcon } from './icons'
import { Message } from './message'

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
        {messages.slice(start).map((message, index) => {
          const i = start + index
          const sent = message.direction === 'sent'
          const received = message.direction === 'received'
          const author = message.sender || message.sender_address || 'Unknown sender'
          const recipients = [...new Set(message.recipients?.map((address) => participant(address)?.name || address))].filter((name) => name !== author)
          const day = formatDate(message.at)
          return (
            <li key={i} className="flex min-w-0 flex-col gap-4">
              {(index === 0 || day !== formatDate(messages[i - 1].at)) && <time dateTime={message.at} className="self-center text-[11.5px] text-ink-3">{day}</time>}
              <div className={cn('flex max-w-[92%] flex-col gap-1.5 sm:max-w-[86%]', received ? 'self-end' : sent ? 'self-start' : 'self-center')}>
                <div className={cn('flex min-w-0 items-center gap-1.5 text-[12px] text-ink-3', received && 'justify-end')}>
                  <RecordIcon object="people" name={author} photo={participant(message.sender_address)?.photo} size={20} />
                  <span className="truncate">
                    <span className="font-medium text-ink-2" title={message.sender_address}>{author}</span>
                    {recipients.length > 0 && <span title={message.recipients?.join(', ')}> to {recipients.join(', ')}</span>}
                  </span>
                </div>
                <div className={cn('min-w-0 rounded-[16px] px-3.5 pb-2 pt-2.5', received ? 'rounded-tr-[4px]' : 'rounded-tl-[4px]', sent ? 'bg-primary-soft' : 'bg-list-hover')}>
                  {message.text ? <Message text={message.text} /> : <p className="text-[13px] text-ink-3">The text arrives with the next sync.</p>}
                  {message.partial && <p className="mt-2 text-[11.5px] text-ink-3">Message excerpt</p>}
                  {message.at.length > 10 && <time dateTime={message.at} title={formatDateTime(message.at)} className="mt-1 block text-right text-[11px] tabular-nums text-ink-3">{formatTime(message.at)}</time>}
                </div>
              </div>
            </li>
          )
        })}
      </ol>
    </section>
  )
}
