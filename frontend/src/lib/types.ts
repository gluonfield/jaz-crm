// Wire shapes of the server's tools; field names are their JSON names.

export const attributeTypes = ['text', 'number', 'date', 'checkbox', 'url', 'select', 'status', 'member', 'email', 'domain', 'phone', 'reference'] as const
export type AttributeType = (typeof attributeTypes)[number]

export type Attribute = {
  slug: string
  name: string
  type: AttributeType
  multi?: boolean
  unique?: boolean
  target?: string
  options?: string[]
}

export type CrmObject = { slug: string; name: string; attributes: Attribute[] }

export type Ref = { id: string; name?: string; photo?: string }

// A value is text, or a referenced record.
export type Value = string | Ref

export type CrmRecord = {
  id: string
  object: string
  created_at: string
  values: Record<string, Value | Value[] | null>
  activity?: { interactions: number; first_at?: string; last_at?: string }
  photo?: string
}

export type Kind = 'email' | 'meeting' | 'call' | 'note'

export type Party = { address: string; name?: string; role: string; person_id?: string; photo?: string }

export type RecordRef = { id: string; object: string; name?: string }

export type Part = { kind: string; at: string; author?: string; content: string }

export type Interaction = {
  id: string
  kind: Kind
  source: string
  title: string
  started_at: string
  ended_at?: string
  participants: Party[]
  records: RecordRef[]
  preview?: string
  parts?: Part[]
}

export type Verdict = 'pending' | 'kept' | 'skipped'

export type Contact = {
  address: string
  kind: string
  name?: string
  photo?: string
  status: string
  decided_by?: string
  reason?: string
  person_id?: string
  interactions: number
  last_seen: string
}

export type Member = { name: string; email: string; admin?: boolean; is_me?: boolean; photo?: string }

export type Workspace = { name: string; description: string; members?: Member[]; invited?: string[] }

export type Connection = {
  id: string
  account: string
  owner_id: string
  status: string
  created_at: string
  synced: Record<string, string>
  step?: string
  backfilled: boolean
  messages: number
  oldest?: string
}

export type Connections = { connections: Connection[]; connect_url?: string; since: string }
