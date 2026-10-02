import { createElement, type ReactNode } from 'react'
import { ExternalLink } from './external-link'

// Elements a signature keeps, by the element they render as; others keep only
// their text, and dropped ones not even that.
const kept: Record<string, string> = {
  B: 'b', STRONG: 'strong', I: 'i', EM: 'em', U: 'u', BR: 'br', SPAN: 'span', FONT: 'span',
  DIV: 'div', P: 'div', TABLE: 'div', TBODY: 'div', TR: 'div', TD: 'span', TH: 'span', UL: 'ul', OL: 'ol', LI: 'li',
}
const dropped = new Set(['SCRIPT', 'STYLE', 'TEMPLATE', 'HEAD', 'TITLE', 'NOSCRIPT', 'IFRAME', 'OBJECT', 'SVG'])

// Signature shows a Gmail signature as Gmail shows it: its text, lines,
// emphasis, links and images, rebuilt as elements rather than injected.
export function Signature({ html }: { html: string }) {
  const body = new DOMParser().parseFromString(html, 'text/html').body
  return <div className="text-[13px] leading-5 text-ink-3 [&_ol]:list-decimal [&_ol]:pl-5 [&_ul]:list-disc [&_ul]:pl-5">{children(body)}</div>
}

function children(node: Node): ReactNode[] {
  return [...node.childNodes].map((child, key) => {
    if (child.nodeType === Node.TEXT_NODE) {
      return child.textContent
    }
    if (!(child instanceof Element) || dropped.has(child.tagName)) {
      return null
    }
    const inner = children(child)
    if (child.tagName === 'A') {
      const href = child.getAttribute('href') ?? ''
      return /^(https?:|mailto:)/i.test(href) ? <ExternalLink key={key} href={href} className="text-ink-2 underline">{inner}</ExternalLink> : <span key={key}>{inner}</span>
    }
    if (child.tagName === 'IMG') {
      const src = child.getAttribute('src') ?? ''
      return src.startsWith('https://') ? <img key={key} src={src} alt={child.getAttribute('alt') ?? ''} referrerPolicy="no-referrer" className="inline-block max-h-16 max-w-full" /> : null
    }
    const tag = kept[child.tagName]
    return tag ? createElement(tag, { key }, ...(tag === 'br' ? [] : inner)) : <span key={key}>{inner}</span>
  })
}
