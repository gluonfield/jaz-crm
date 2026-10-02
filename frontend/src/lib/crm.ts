import { hasEnded } from './format'
import type { CrmRecord, Interaction, Value } from './types'

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

// contextHint suggests a person's context as a bullet-point TLDR, the form
// agents keep it in.
export const contextHint = '- Who they are and how you know them\n- 2026-10-01: what happened'

const joinLink = /https?:\/\/(teams\.microsoft\.com|[\w.-]*zoom\.us|meet\.google\.com)\/\S+/

// joinURL is how to join a meeting that has not ended: its Google Meet call,
// else a call link in its invitation.
export function joinURL(interaction: Interaction) {
  if (interaction.kind !== 'meeting' || hasEnded(interaction.ended_at ?? interaction.started_at)) {
    return undefined
  }
  return interaction.meet_url ?? interaction.invitation?.match(joinLink)?.[0]
}
