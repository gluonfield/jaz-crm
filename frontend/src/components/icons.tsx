import { Box, Building2, CalendarDays, Handshake, Mail, NotebookPen, Phone, Users } from 'lucide-react'
import { useState } from 'react'
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

// RecordIcon draws a person as a round avatar and anything else as a square
// tile: the profile picture when there is one, else lettered and tinted by
// name.
export function RecordIcon({ object, name, photo, size = 18, className }: { object: string; name: string; photo?: string; size?: number; className?: string }) {
  const [failed, setFailed] = useState<string>()
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

const objectIcons: Record<string, typeof Box> = { people: Users, companies: Building2, deals: Handshake }

export function ObjectIcon({ slug, className }: { slug: string; className?: string }) {
  const Icon = objectIcons[slug] ?? Box
  return <Icon className={className} />
}

const kindIcons = { email: Mail, meeting: CalendarDays, call: Phone, note: NotebookPen }

export function KindIcon({ kind, className }: { kind: Kind; className?: string }) {
  const Icon = kindIcons[kind] ?? NotebookPen
  return <Icon className={cn('size-3.5 shrink-0', className)} />
}
