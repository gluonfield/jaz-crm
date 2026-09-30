import type { Attribute } from './types'

// Stage colours follow a status's order, so a pipeline reads left to right:
// the first four suit Lead, In progress, Won and Lost.
const palette = [1, 4, 3, 5, 2, 6]

export function stageColor(attribute: Attribute, stage: string) {
  const index = Math.max(0, (attribute.options ?? []).indexOf(stage))
  return `var(--color-avatar-${palette[index % palette.length]})`
}

// statusOf is the attribute an object's pipeline moves along, if it has one.
export const statusOf = (attributes: Attribute[]) => attributes.find((a) => a.type === 'status')
