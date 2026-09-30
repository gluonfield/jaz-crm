import { Fragment, type ReactNode } from 'react'
import { embedded } from '@/lib/api'
import { app } from '@/lib/mcp-app'
import { cn } from '@/lib/utils'

// Message renders text the server cleaned: paragraphs, "- " lists and bare
// links, which show as their site and a short path.
export function Message({ text, className }: { text: string; className?: string }) {
  return (
    <div className={cn('flex max-w-[68ch] flex-col gap-3 break-words text-[13.5px] leading-[1.6] text-ink-2', className)}>
      {text.split(/\n{2,}/).map((block, i) => (
        <Block key={i} lines={block.split('\n')} />
      ))}
    </div>
  )
}

// Block splits a paragraph into runs of list items and plain lines.
function Block({ lines }: { lines: string[] }) {
  const runs: { list: boolean; lines: string[] }[] = []
  for (const line of lines) {
    const list = line.startsWith('- ')
    const last = runs.at(-1)
    if (last?.list === list) {
      last.lines.push(list ? line.slice(2) : line)
    } else {
      runs.push({ list, lines: [list ? line.slice(2) : line] })
    }
  }
  return runs.map((run, i) =>
    run.list ? (
      <ul key={i} className="flex list-disc flex-col gap-1 pl-5 marker:text-ink-3">
        {run.lines.map((line, j) => (
          <li key={j}>
            <Inline text={line} />
          </li>
        ))}
      </ul>
    ) : (
      <p key={i}>
        {run.lines.map((line, j) => (
          <Fragment key={j}>
            {j > 0 && <br />}
            <Inline text={line} />
          </Fragment>
        ))}
      </p>
    ),
  )
}

// Inline turns the web addresses in a line into links, leaving punctuation
// that ends a sentence outside them.
function Inline({ text }: { text: string }) {
  return text.split(/(https?:\/\/\S+)/g).map((piece, i) => {
    if (i % 2 === 0) {
      return piece
    }
    const tail = piece.match(/[.,;:!?)'"\]]+$/)?.[0] ?? ''
    return (
      <Fragment key={i}>
        <ExternalLink href={piece.slice(0, piece.length - tail.length)} />
        {tail}
      </Fragment>
    )
  })
}

// shortLink shows an address as its site and the start of its path.
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

export function ExternalLink({ href, children, className }: { href: string; children?: ReactNode; className?: string }) {
  return (
    <a
      href={href}
      title={href}
      target="_blank"
      rel="noopener noreferrer"
      onClick={(e) => {
        if (embedded()) {
          e.preventDefault()
          void app.openLink({ url: href })
        }
      }}
      className={cn('text-ink underline decoration-ink-3/50 underline-offset-2 transition-colors hover:decoration-ink', className)}
    >
      {children ?? shortLink(href)}
    </a>
  )
}
