import type { ReactNode } from 'react'
import { embedded } from '@/lib/api'
import { app } from '@/lib/mcp-app'
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
