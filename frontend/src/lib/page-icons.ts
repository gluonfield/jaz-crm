import { icons } from 'lucide-react'
import emojiCatalog from './emoji-catalog.json'

export { icons as pageSymbols }
export const pageSymbolNames = Object.keys(icons) as (keyof typeof icons)[]
export const pageEmojis = emojiCatalog.map(([value, label]) => ({ value, label }))
export const iconColors = ['default', 'gray', 'brown', 'orange', 'yellow', 'green', 'blue', 'purple', 'pink', 'red'] as const
export type IconColor = typeof iconColors[number]

const aliases: Record<string, keyof typeof icons> = {
  briefcase: 'BriefcaseBusiness', building: 'BuildingComplex', calendar: 'CalendarDays', chart: 'ChartNoAxesCombined',
  checklist: 'ClipboardList', code: 'CodeXml', file: 'FileText', message: 'MessageSquare', team: 'Users', check: 'CircleCheck',
}

export function pageSymbol(value?: string) {
  if (!value?.startsWith('icon:')) {
    return undefined
  }
  const [saved, color] = value.slice(5).split(':')
  const name = aliases[saved] ?? saved.replace(/(^|-)([a-z])/g, (_, _prefix, letter: string) => letter.toUpperCase())
  return {
    name: Object.hasOwn(icons, name) ? name as keyof typeof icons : undefined,
    color: iconColors.find((candidate) => candidate === color) ?? 'default',
  }
}

export const symbolValue = (name: string, color: IconColor) => `icon:${name}${color === 'default' ? '' : `:${color}`}`
export const colorValue = (color: IconColor) => color === 'default' ? 'var(--color-ink-2)' : `var(--page-icon-${color})`
