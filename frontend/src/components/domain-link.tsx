import { ExternalLink } from './external-link'

export function DomainLink({ domain, className }: { domain: string; className?: string }) {
  return (
    <ExternalLink href={`https://${domain}`} className={className}>
      {domain}
    </ExternalLink>
  )
}
