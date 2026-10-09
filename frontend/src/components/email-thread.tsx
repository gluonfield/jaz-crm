import { Button } from '@jaz/ui/button'
import { ArrowDownLeft, ArrowUpRight, ChevronDown } from 'lucide-react'
import { type MouseEvent, type ReactNode, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { formatDate, formatDateTime, formatTime, timeAgo } from '@/lib/format'
import type { Interaction, CrmMessage } from '@/lib/types'
import { cn } from '@/lib/utils'
import { ChannelIcon, RecordIcon } from './icons'
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

const authorOf = (message: CrmMessage) => message.sender || message.sender_address || 'Unknown sender'

// An event is something that happened in a conversation besides a message,
// such as a follow-up being created.
export type ThreadEvent = { at: string; label: ReactNode }

type Item = { day: string; author?: string; messages: CrmMessage[]; event?: ReactNode }

// items groups consecutive messages from one sender on one day, as a chat
// shows them, with events in their place between the messages.
function items(messages: CrmMessage[], events: ThreadEvent[]) {
  const out: Item[] = []
  const pending = [...events].sort((a, b) => Date.parse(a.at) - Date.parse(b.at))
  const happen = (before = Infinity) => {
    while (pending.length > 0 && Date.parse(pending[0].at) < before) {
      const event = pending.shift()!
      out.push({ day: formatDate(event.at), messages: [], event: event.label })
    }
  }
  for (const message of messages) {
    happen(Date.parse(message.at))
    const day = formatDate(message.at)
    const author = authorOf(message)
    const run = out.at(-1)
    if (run?.day === day && run.author === author) {
      run.messages.push(message)
    } else {
      out.push({ day, author, messages: [message] })
    }
  }
  happen()
  return out
}

// Divider centres a label, such as a day, between messages, as chat apps do.
export function Divider({ children }: { children: ReactNode }) {
  return <p className="text-center text-[11.5px] font-medium text-ink-3">{children}</p>
}

// Happening is something that happened in a conversation besides a message.
export function Happening({ children }: { children: ReactNode }) {
  return <p className="flex items-center gap-2.5 pl-9 text-[12.5px] text-ink-3"><span aria-hidden="true" className="size-1.5 shrink-0 rounded-full bg-primary" />{children}</p>
}

// MessageThread shows email as a mail reader does, one card per message,
// and other channels as a chat.
export function MessageThread(props: { interaction: Interaction; messages: CrmMessage[]; events?: ThreadEvent[]; initialVisible?: number }) {
  return !props.interaction.channel || props.interaction.channel === 'email' ? <MailThread {...props} /> : <ChatThread {...props} />
}

type Party = (address?: string) => Interaction['participants'][number] | undefined

// recipientsOf names whom a message went to, leaving out its author.
const recipientsOf = (message: CrmMessage, party: Party) => [...new Set(message.recipients?.map((address) => party(address)?.name || address))].filter((name) => name !== authorOf(message))

function MessageBody({ message, className }: { message: CrmMessage; className?: string }) {
  return <>
    <span className="sr-only">{authorOf(message)}: </span>
    {message.text || message.html ? <Message text={message.text} html={message.html} className={className} /> : <p className="text-[13px] text-ink-3">The text arrives with the next sync.</p>}
    {message.partial && <p className="mt-2 text-[11.5px] text-ink-3">Message excerpt</p>}
  </>
}

// MailThread opens the latest two messages and folds the earlier ones to a
// line each, folding a long middle away entirely, as mail readers do.
function MailThread({ interaction, messages, events = [] }: { interaction: Interaction; messages: CrmMessage[]; events?: ThreadEvent[] }) {
  const last = messages.length - 1
  const [open, setOpen] = useState<Record<number, boolean>>({})
  const [all, setAll] = useState(false)
  const section = useRef<HTMLElement>(null)
  const party: Party = (address) => interaction.participants.find((p) => p.address === address)
  const opened = (i: number) => open[i] ?? i >= last - 1
  const folded = (i: number) => !all && last > 4 && i > 0 && i < last - 2
  const toggle = (i: number) => setOpen({ ...open, [i]: !opened(i) })
  const entries = [
    ...messages.map((message, i) => ({ at: Date.parse(message.at), i })),
    ...events.map((event) => ({ at: Date.parse(event.at), event: event.label })),
  ].sort((a, b) => a.at - b.at)
  return (
    <section ref={section} aria-label="Messages" className="flex min-w-0 flex-col gap-2">
      {entries.map((entry, k) => {
        if ('event' in entry) {
          return <div key={`e${k}`} className="py-1"><Happening>{entry.event}</Happening></div>
        }
        const { i } = entry
        if (folded(i)) {
          return i === 1 && (
            <button key="more" type="button" onClick={() => {
              setAll(true)
              requestAnimationFrame(() => section.current?.querySelector<HTMLElement>('[data-message="1"]')?.focus())
            }} className="group flex items-center gap-3 py-0.5 text-[12px] text-ink-3 outline-none before:h-px before:flex-1 before:bg-border after:h-px after:flex-1 after:bg-border hover:text-ink focus-visible:text-ink">
              <span className="rounded-full border border-border px-2.5 py-0.5 tabular-nums group-focus-visible:bg-list-active">{last - 3} more messages</span>
            </button>
          )
        }
        const message = messages[i]
        const author = authorOf(message)
        const photo = party(message.sender_address)?.photo
        if (!opened(i)) {
          return (
            <button key={i} data-message={i} type="button" aria-expanded={false} onClick={() => toggle(i)} className="flex h-10 w-full min-w-0 items-center gap-2.5 rounded-[var(--radius-card)] bg-list-hover px-3.5 text-left text-[13px] outline-none transition-colors duration-150 hover:bg-list-active focus-visible:bg-list-active">
              <RecordIcon object="people" name={author} photo={photo} size={22} />
              <span className="w-36 shrink-0 truncate font-medium text-ink" title={message.sender_address}>{author}</span>
              <span className="min-w-0 flex-1 truncate text-ink-3">{message.text.replace(/\s+/g, ' ')}</span>
              <time dateTime={message.at} className="shrink-0 text-[12px] tabular-nums text-ink-3">{formatDate(message.at)}</time>
            </button>
          )
        }
        const recipients = recipientsOf(message, party)
        const heading = <>
          <RecordIcon object="people" name={author} photo={photo} size={26} />
          <span className="min-w-0 shrink truncate text-[13px] font-medium text-ink" title={message.sender_address}>{author}</span>
          {recipients.length > 0 && <span className="min-w-0 flex-1 truncate text-[12px] text-ink-3">to {recipients.join(', ')}</span>}
          <time dateTime={message.at} className="ml-auto shrink-0 text-[12px] tabular-nums text-ink-3">{formatDateTime(message.at)}</time>
        </>
        return (
          <article key={i} className="rounded-[var(--radius-card)] bg-list-hover px-3.5 py-3">
            {i === last
              ? <div className="flex min-w-0 items-center gap-2.5">{heading}</div>
              : <button type="button" data-message={i} aria-expanded onClick={() => toggle(i)} className="-m-1 flex w-[calc(100%+8px)] min-w-0 items-center gap-2.5 rounded-[var(--radius-control)] p-1 text-left outline-none focus-visible:bg-list-active">{heading}</button>}
            <div className="mt-2 pl-9"><MessageBody message={message} className="text-ink" /></div>
          </article>
        )
      })}
    </section>
  )
}

function ChatThread({ interaction, messages, events = [], initialVisible = 6 }: { interaction: Interaction; messages: CrmMessage[]; events?: ThreadEvent[]; initialVisible?: number }) {
  const [expanded, setExpanded] = useState(false)
  const start = expanded ? 0 : Math.max(0, messages.length - initialVisible)
  const party: Party = (address) => interaction.participants.find((p) => p.address === address)
  const channel = interaction.channel ?? ''
  return (
    <section aria-label="Messages" className="flex min-w-0 flex-col gap-3.5">
      {messages.length > initialVisible && (
        <Button onClick={() => setExpanded(!expanded)} variant="ghost" size="sm" aria-expanded={expanded} className="self-center">
          {expanded ? 'Hide earlier messages' : <>Show thread<span className="tabular-nums text-ink-3">· {messages.length - initialVisible} earlier</span></>}
          <ChevronDown aria-hidden="true" className={cn('transition-transform duration-150 motion-reduce:transition-none', expanded && 'rotate-180')} />
        </Button>
      )}
      <ol className="flex min-w-0 flex-col gap-3.5">
        {items(messages.slice(start), events).map((item, i, all) => {
          const day = item.day !== all[i - 1]?.day && <Divider><time dateTime={item.messages[0]?.at}>{item.day}</time></Divider>
          if (item.event) {
            return (
              <li key={i} className="flex min-w-0 flex-col gap-3.5">
                {day}
                <Happening>{item.event}</Happening>
              </li>
            )
          }
          const author = item.author!
          const first = item.messages[0]
          const latest = item.messages[item.messages.length - 1]
          const sent = latest.direction === 'sent'
          const recipients = recipientsOf(latest, party)
          return (
            <li key={i} className="flex min-w-0 flex-col gap-3.5">
              {day}
              <div className={cn('flex max-w-[88%] items-start gap-2.5 sm:max-w-[min(76%,620px)]', sent ? 'self-end' : 'self-start')}>
                <CursorTip content={<>
                  <span className="font-medium">{author}</span>
                  {latest.sender_address !== author && <span>{latest.sender_address}</span>}
                  {recipients.length > 0 && <span>to {recipients.join(', ')}</span>}
                </>}>
                  <RecordIcon object="people" name={author} photo={party(latest.sender_address)?.photo} size={26} />
                </CursorTip>
                <div className={cn('flex min-w-0 flex-col gap-1', sent ? 'items-end' : 'items-start')}>
                  <p className="flex min-w-0 max-w-full items-center gap-1.5 text-[12px] text-ink-3">
                    <span className="truncate">{author}</span>
                    {first.at.length > 10 && <time dateTime={first.at} title={formatDateTime(first.at)} className="shrink-0 tabular-nums">· {formatTime(first.at)}</time>}
                    {channel && <ChannelIcon channel={channel} className="size-3" />}
                  </p>
                  {item.messages.map((message, j) => (
                    <div key={j} title={message.at.length > 10 ? formatDateTime(message.at) : undefined} className={cn('min-w-0 max-w-full rounded-[12px] px-3.5 py-2.5 first-of-type:rounded-tl-[4px]', sent ? 'bg-primary-soft' : 'bg-list-hover')}>
                      <MessageBody message={message} />
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

// CursorTip shows its content beside the pointer as soon as it arrives and
// follows it, turning away from the window's far edges.
function CursorTip({ content, children }: { content: ReactNode; children: ReactNode }) {
  const [at, setAt] = useState<{ x: number; y: number } | null>(null)
  const follow = (e: MouseEvent) => setAt({ x: e.clientX, y: e.clientY })
  return (
    <span className="shrink-0" onMouseEnter={follow} onMouseMove={follow} onMouseLeave={() => setAt(null)}>
      {children}
      {at && createPortal(
        <div
          role="tooltip"
          style={{ ...(at.x < innerWidth / 2 ? { left: at.x + 12 } : { right: innerWidth - at.x + 12 }), ...(at.y < innerHeight - 96 ? { top: at.y + 16 } : { bottom: innerHeight - at.y + 12 }) }}
          className="pointer-events-none fixed z-50 flex flex-col rounded-md bg-foreground px-3 py-1.5 text-xs text-background"
        >
          {content}
        </div>,
        document.body,
      )}
    </span>
  )
}
