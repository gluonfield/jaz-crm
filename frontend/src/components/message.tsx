import { Fragment } from 'react'
import { cn } from '@/lib/utils'
import { ExternalLink } from './external-link'
import { HTMLContent } from './html-content'

export function Message({ text, html, className }: { text: string; html?: string; className?: string }) {
  if (html) {
    return <HTMLContent html={html} className={cn('text-[13.5px] leading-[1.6] text-ink-2', className)} />
  }
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
        <ExternalLink href={piece.slice(0, piece.length - tail.length)} className="text-ink" />
        {tail}
      </Fragment>
    )
  })
}
