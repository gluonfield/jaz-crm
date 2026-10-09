import type { ComponentType, ReactNode } from 'react'
import { embedded } from '@/lib/api'
import { app } from '@/lib/mcp-app'
import type { Attribute } from '@/lib/types'
import { cn } from '@/lib/utils'

export function ExternalLink({ href, children, className }: { href: string; children?: ReactNode; className?: string }) {
  return (
    <a
      href={href}
      title={href}
      target="_blank"
      rel="noopener noreferrer"
      onKeyDown={(e) => e.stopPropagation()}
      onClick={(e) => {
        e.stopPropagation()
        if (embedded()) {
          e.preventDefault()
          void app.openLink({ url: href })
        }
      }}
      className={cn('decoration-dotted underline-offset-2 hover:text-ink hover:underline', className)}
    >
      {children ?? shortLink(href)}
    </a>
  )
}

export function DomainLink({ value, className }: { value: string; className?: string }) {
  return (
    <ExternalLink href={`https://${value}`} className={className}>
      {value}
    </ExternalLink>
  )
}

// valueLinks show values of the link types as links to the pages they name.
export const valueLinks: Partial<Record<Attribute['type'], ComponentType<{ value: string; className?: string }>>> = {
  url: ({ value, className }) => <ExternalLink href={value} className={className} />,
  domain: DomainLink,
}

function shortLink(href: string) {
  try {
    const url = new URL(href)
    const path = url.pathname === '/' ? '' : url.pathname
    const site = url.hostname.replace(/^www\./, '')
    return path.length > 24 ? `${site}${path.slice(0, 22)}…` : `${site}${path}${url.search ? '…' : ''}`
  } catch {
    return href
  }
}
