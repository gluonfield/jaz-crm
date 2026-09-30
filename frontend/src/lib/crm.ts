import type { CrmRecord, Value } from './types'

// valuesOf lists an attribute's current values, whether it holds one or many.
export function valuesOf(record: CrmRecord, slug: string): Value[] {
  const value = record.values[slug]
  return value == null ? [] : Array.isArray(value) ? value : [value]
}

export const valueText = (value: Value) => (typeof value === 'string' ? value : (value.name ?? value.id))

// valueKey is how a write names a value: its text, or a referenced record's id.
export const valueKey = (value: Value) => (typeof value === 'string' ? value : value.id)

// recordName falls back to the first text value, such as an email address.
export function recordName(record: CrmRecord) {
  const name = valuesOf(record, 'name')[0] ?? Object.values(record.values).flat().find((v) => typeof v === 'string')
  return name ? valueText(name) : 'Unnamed'
}

export const slugify = (name: string) =>
  name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^[^a-z]+|_+$/g, '')
    .slice(0, 40)
