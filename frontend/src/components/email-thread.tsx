import { Button } from '@jaz/ui/button'
import { ArrowDownLeft, ArrowUpRight } from 'lucide-react'
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

export function MessageThread({ interaction, messages }: { interaction: Interaction; messages: CrmMessage[] }) {
  const [visible, setVisible] = useState(6)
  const start = Math.max(0, messages.length - visible)
  return (
    <section aria-label="Messages" className="flex min-w-0 flex-col gap-6">
      {start > 0 && (
        <Button onClick={() => setVisible((count) => count + 20)} variant="ghost" className="self-center">
          Show {start} earlier {start === 1 ? 'message' : 'messages'}
        </Button>
      )}
      <ol className="flex min-w-0 flex-col gap-4">
        {messages.slice(start).map((message, index) => {
          const i = start + index
          const sent = message.direction === 'sent'
          const author = message.sender || message.sender_address || 'Unknown sender'
          const person = interaction.participants.find((p) => p.address === message.sender_address)
          const day = formatDate(message.at)
          return (
            <li key={i} className="flex min-w-0 flex-col gap-4">
              {(index === 0 || day !== formatDate(messages[i - 1].at)) && <time dateTime={message.at} className="self-center text-[11.5px] text-ink-3">{day}</time>}
              <div className={cn('flex max-w-[92%] flex-col gap-1.5 sm:max-w-[86%]', sent ? 'self-end' : message.direction === 'received' ? 'self-start' : 'self-center')}>
                <div className={cn('flex min-w-0 items-center gap-1.5 text-[12px]', sent && 'justify-end')}>
                  <RecordIcon object="people" name={author} photo={person?.photo} size={20} />
                  <span className="truncate font-medium text-ink-2" title={message.sender_address}>{author}</span>
                  {message.direction && <span className="shrink-0 text-ink-3">{sent ? 'Sent' : 'Received'}</span>}
                </div>
                {message.recipients && <p className="text-[11.5px] text-ink-3">To {message.recipients.join(', ')}</p>}
                <div className={cn('min-w-0 rounded-[16px] px-4 py-3', sent ? 'rounded-br-[4px] bg-primary-soft' : 'rounded-bl-[4px] bg-list-hover')}>
                  {message.text ? <Message text={message.text} /> : <p className="text-[13px] text-ink-3">The text arrives with the next sync.</p>}
                  {message.partial && <p className="mt-2 text-[11.5px] text-ink-3">Message excerpt</p>}
                  {message.at.length > 10 && <time dateTime={message.at} title={formatDateTime(message.at)} className="mt-2 block text-right text-[11px] tabular-nums text-ink-3">{formatTime(message.at)}</time>}
                </div>
              </div>
            </li>
          )
        })}
      </ol>
    </section>
  )
}
