import { mergeAttributes, Node } from '@tiptap/core'
import { NodeViewContent, NodeViewWrapper, type ReactNodeViewProps, ReactNodeViewRenderer } from '@tiptap/react'
import { SmilePlus } from 'lucide-react'
import type { CSSProperties } from 'react'
import { colorValue, type IconColor, iconColors } from '@/lib/page-icons'
import { IconPicker, PageIcon } from './page-icon'

// A callout is stored as an Obsidian-style quote, which other markdown
// renderers show as a quote: > [!callout|blue] 💡, then its body.
const header = /^ {0,3}> ?\[!callout(?:\|([a-z]+))?\][ \t]*(\S*)[ \t]*(?:\n|$)((?:>.*(?:\n|$))*)/i

// tint colors an element with the callout-tint class in a callout's color.
export const tint = (color: IconColor) => ({ '--callout': colorValue(color) }) as CSSProperties

function CalloutView({ node, editor, updateAttributes }: ReactNodeViewProps) {
  const { icon, color } = node.attrs as { icon: string; color: IconColor }
  const face = icon ? <PageIcon value={icon} size={20} /> : <SmilePlus className="size-4" />
  return (
    <NodeViewWrapper as="aside" data-callout="" className="callout-tint" style={tint(color)}>
      {editor.isEditable ? (
        <IconPicker value={icon || undefined} pending={false} onChange={(next, saved) => {
          updateAttributes({ icon: next ?? '' })
          saved()
        }}>
          <button type="button" contentEditable={false} aria-label={icon ? 'Change callout icon' : 'Add callout icon'} data-empty={!icon || undefined} className="callout-icon">{face}</button>
        </IconPicker>
      ) : icon && <span contentEditable={false} className="callout-icon">{face}</span>}
      <NodeViewContent className="min-w-0 flex-1" />
    </NodeViewWrapper>
  )
}

export const Callout = Node.create({
  name: 'callout',
  group: 'block',
  content: 'block+',
  defining: true,
  addAttributes() {
    return {
      icon: { default: '💡', parseHTML: (element) => element.getAttribute('data-icon') ?? '', renderHTML: ({ icon }) => ({ 'data-icon': icon }) },
      color: { default: 'default', parseHTML: (element) => element.getAttribute('data-color') ?? 'default', renderHTML: ({ color }) => ({ 'data-color': color }) },
    }
  },
  parseHTML: () => [{ tag: 'aside[data-callout]' }],
  renderHTML: ({ HTMLAttributes }) => ['aside', mergeAttributes(HTMLAttributes, { 'data-callout': '' }), 0],
  addNodeView: () => ReactNodeViewRenderer(CalloutView),
  markdownTokenizer: {
    name: 'callout',
    level: 'block',
    start: (src) => src.search(/> ?\[!callout/i),
    tokenize: (src, _tokens, lexer) => {
      const match = header.exec(src)
      if (match) {
        const color = iconColors.find((choice) => choice === match[1]?.toLowerCase()) ?? 'default'
        return { type: 'callout', raw: match[0], color, icon: match[2], tokens: lexer.blockTokens(match[3].replace(/^> ?/gm, '')) }
      }
    },
  },
  parseMarkdown: (token, helpers) => {
    const content = (helpers.parseBlockChildren ?? helpers.parseChildren)(token.tokens ?? [])
    return helpers.createNode('callout', { icon: token.icon, color: token.color }, content.length ? content : [{ type: 'paragraph' }])
  },
  renderMarkdown: (node, helpers) => {
    const { icon, color } = node.attrs as { icon: string; color: IconColor }
    const marker = `> [!callout${color === 'default' ? '' : `|${color}`}]${icon ? ` ${icon}` : ''}`
    const body = (node.content ?? []).map((child) => helpers.renderChildren([child]).split('\n').map((line) => line.trim() ? `> ${line}` : '>').join('\n'))
    return `${marker}\n${body.join('\n>\n')}`
  },
})
