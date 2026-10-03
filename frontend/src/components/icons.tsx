import { Building2, CalendarDays, CornerUpRight, FileText, Handshake, MessageCircle, MessageSquare, NotebookPen, Phone, Table2, Users } from 'lucide-react'
import { useState } from 'react'
import { channelNames } from '@/lib/crm'
import type { Kind } from '@/lib/types'
import { cn } from '@/lib/utils'

function hash(value: string) {
  let h = 0
  for (const char of value) {
    h = (h * 31 + char.charCodeAt(0)) | 0
  }
  return Math.abs(h)
}

function initials(name: string) {
  const words = name.replace(/[^\p{L}\p{N} ]/gu, ' ').trim().split(/\s+/)
  return ((words[0]?.[0] ?? '?') + (words.length > 1 ? words[words.length - 1][0] : '')).toUpperCase()
}

// RecordIcon draws a page as a document, a person as a round avatar and
// anything else as a square tile: the profile picture when there is one, else
// lettered and tinted by name.
export function RecordIcon({ object, name, photo, size = 18, className }: { object: string; name: string; photo?: string; size?: number; className?: string }) {
  const [failed, setFailed] = useState<string>()
  if (object === 'pages') {
    return <FileText aria-hidden style={{ width: Math.min(size, 20), height: Math.min(size, 20) }} className={cn('shrink-0 text-ink-2', className)} />
  }
  const shape = object === 'people' ? 'rounded-full' : 'rounded-[28%]'
  if (photo && failed !== photo) {
    return (
      <img
        src={photo}
        alt=""
        referrerPolicy="no-referrer"
        onError={() => setFailed(photo)}
        style={{ width: size, height: size }}
        className={cn(
          'inline-block shrink-0 object-cover',
          // Logos are often transparent and dark, so they sit on white.
          object === 'people' ? 'bg-list-active' : 'bg-white object-contain outline outline-1 -outline-offset-1 outline-black/10',
          shape,
          className,
        )}
      />
    )
  }
  return (
    <span
      style={{ width: size, height: size, fontSize: size * 0.42, background: `var(--color-avatar-${(hash(name) % 6) + 1})` }}
      className={cn('inline-flex shrink-0 select-none items-center justify-center font-semibold leading-none text-avatar-ink', shape, className)}
      aria-hidden
    >
      {object === 'people' ? initials(name) : initials(name)[0]}
    </span>
  )
}

const objectIcons: Record<string, typeof Table2> = { people: Users, companies: Building2, deals: Handshake, follow_ups: CornerUpRight, pages: FileText }

export function ObjectIcon({ slug, className }: { slug: string; className?: string }) {
  const Icon = objectIcons[slug] ?? Table2
  return <Icon className={className} />
}

const kindIcons = { message: MessageSquare, meeting: CalendarDays, call: Phone, note: NotebookPen }

export function KindIcon({ kind, className }: { kind: Kind; className?: string }) {
  const Icon = kindIcons[kind] ?? NotebookPen
  return <Icon className={cn('size-3.5 shrink-0', className)} />
}

const linkedin = 'M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433a2.062 2.062 0 1 1 0-4.125 2.062 2.062 0 0 1 0 4.125zM7.119 20.452H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z'

// ChannelIcon marks a message that came by something other than email: its
// service's mark where we draw one, else a speech bubble.
export function ChannelIcon({ channel, className }: { channel: string; className?: string }) {
  return (
    <span title={`Via ${channelNames[channel] ?? channel}`} className={cn('inline-flex shrink-0', className)}>
      {channel === 'linkedin' ? <svg aria-hidden="true" viewBox="0 0 24 24" fill="currentColor" className="size-full"><path d={linkedin} /></svg> : <MessageCircle aria-hidden="true" className="size-full" />}
    </span>
  )
}

// ChannelTag names the channel a conversation is on, LinkedIn in its blue.
export function ChannelTag({ channel }: { channel: string }) {
  return (
    <span className={cn('shrink-0 rounded-[4px] px-[5px] text-[11px] leading-4', channel === 'linkedin' ? 'bg-[#0a66c2]/12 text-[#0a66c2] dark:bg-[#0a66c2]/25 dark:text-[#8fb4e8]' : 'bg-list-active text-ink-2')}>
      {channelNames[channel] ?? channel}
    </span>
  )
}
