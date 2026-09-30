import type { Attribute } from './types'

const palette = [1, 4, 3, 5, 2, 6]
const standard: Record<string, number> = { lead: 1, 'in progress': 4, won: 3, lost: 5 }

export function stageColor(stage: string) {
  const name = stage.toLowerCase()
  const hash = [...name].reduce((value, letter) => (value * 31 + letter.codePointAt(0)!) % palette.length, 0)
  return `var(--color-avatar-${standard[name] ?? palette[hash]})`
}

// statusOf is the attribute an object's pipeline moves along, if it has one.
export const statusOf = (attributes: Attribute[]) => attributes.find((a) => a.type === 'status')
