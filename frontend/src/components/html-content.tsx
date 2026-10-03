import { createElement, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { ExternalLink } from './external-link'

const kept: Record<string, string> = {
  B: 'b', STRONG: 'strong', I: 'i', EM: 'em', U: 'u', S: 's', BR: 'br', HR: 'hr', SPAN: 'span', FONT: 'span',
  DIV: 'div', P: 'p', TABLE: 'table', THEAD: 'thead', TBODY: 'tbody', TFOOT: 'tfoot', TR: 'tr', TD: 'td', TH: 'th',
  UL: 'ul', OL: 'ol', LI: 'li', BLOCKQUOTE: 'blockquote', PRE: 'pre', CODE: 'code',
}
const dropped = new Set(['SCRIPT', 'STYLE', 'TEMPLATE', 'HEAD', 'TITLE', 'NOSCRIPT', 'IFRAME', 'OBJECT', 'EMBED', 'SVG', 'MATH', 'FORM'])
const hidden = (el: Element) => dropped.has(el.tagName) || el.classList.contains('gmail_quote') || el.classList.contains('yahoo_quoted') || (el.tagName === 'BLOCKQUOTE' && el.getAttribute('type') === 'cite')
const blank = (node: Node): boolean => node instanceof Element
  ? hidden(node) || (node.tagName !== 'IMG' && !node.textContent?.trim() && !node.querySelector('img'))
  : node.nodeType !== Node.TEXT_NODE || !node.textContent?.trim()

// Rebuild untrusted email HTML as elements without copying styles or event attributes.
export function HTMLContent({ html, className }: { html: string; className?: string }) {
  const body = new DOMParser().parseFromString(html, 'text/html').body
  cutQuote(body)
  trimEnd(body)
  return <div className={cn('max-w-[68ch] text-[13px] leading-5 text-ink-3 [overflow-wrap:anywhere] [&_ol]:list-decimal [&_ol]:pl-5 [&_ul]:list-disc [&_ul]:pl-5 [&_p]:mb-3 [&_p:last-child]:mb-0 [&_table]:max-w-full [&_td]:align-top [&_th]:align-top [&_pre]:whitespace-pre-wrap', className)}>{children(body)}</div>
}

// cutQuote removes the earlier mail quoted below a reply, as chat apps hide it:
// from Outlook's reply markers, or a header that starts "From:" and names the
// subject. A forward with nothing written above it keeps its content.
function cutQuote(body: HTMLElement) {
  const start = body.querySelector('#mail-editor-reference-message-container, #divRplyFwdMsg, #appendonsend')
    ?? [...body.querySelectorAll('div, p')].find((el) => /^From:/.test(el.textContent?.trim() ?? '') && /Subject:/.test(after(el, body).toString().slice(0, 400)))
  if (!start) {
    return
  }
  const before = document.createRange()
  before.setStart(body, 0)
  before.setEndBefore(start)
  if (before.toString().trim()) {
    after(start, body).deleteContents()
  }
}

// after is the content from node to the end of body.
function after(node: Node, body: HTMLElement) {
  const range = document.createRange()
  range.setStartBefore(node)
  range.setEndAfter(body.lastChild!)
  return range
}

// trimEnd drops the empty lines and hidden quotes that would pad a message's end.
function trimEnd(node: Node) {
  while (node.lastChild && blank(node.lastChild)) {
    node.lastChild.remove()
  }
  if (node.lastChild) {
    trimEnd(node.lastChild)
  }
}

function children(node: Node): ReactNode[] {
  return [...node.childNodes].map((child, key) => {
    if (child.nodeType === Node.TEXT_NODE) {
      return child.textContent
    }
    if (!(child instanceof Element) || hidden(child)) {
      return null
    }
    const inner = children(child)
    if (child.tagName === 'A') {
      const href = child.getAttribute('href') ?? ''
      return /^(https?:|mailto:|tel:)/i.test(href) ? <ExternalLink key={key} href={href} className="underline">{inner}</ExternalLink> : <span key={key}>{inner}</span>
    }
    if (child.tagName === 'IMG') {
      const src = child.getAttribute('src') ?? ''
      return src.startsWith('https://') ? <img key={key} src={src} alt={child.getAttribute('alt') ?? ''} referrerPolicy="no-referrer" className="inline-block max-w-full" /> : null
    }
    const tag = kept[child.tagName]
    return tag ? createElement(tag, { key }, ...(tag === 'br' || tag === 'hr' ? [] : inner)) : <span key={key}>{inner}</span>
  })
}
