import type { ComponentProps } from 'react'
import { cn } from '@/lib/utils'

// DomainLink opens a company's site. It reads as plain text until hovered,
// when a dotted underline shows it can be followed.
export function DomainLink({ domain, className, ...props }: { domain: string } & ComponentProps<'a'>) {
  return (
    <a href={`https://${domain}`} target="_blank" rel="noreferrer" {...props} className={cn('decoration-dotted underline-offset-2 hover:text-ink hover:underline', className)}>
      {domain}
    </a>
  )
}
