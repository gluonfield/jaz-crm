// Wire shapes of the server's tools; field names are their JSON names.

// attributeTypes are the types a new attribute can take; markdown is the
// content of pages and table records.
export const attributeTypes = ['text', 'number', 'date', 'datetime', 'checkbox', 'url', 'select', 'status', 'member', 'email', 'domain', 'phone', 'reference'] as const
export type AttributeType = (typeof attributeTypes)[number] | 'markdown'

export type Attribute = {
  slug: string
  name: string
  type: AttributeType
  multi?: boolean
  unique?: boolean
  target?: string
  options?: string[]
}

export type CrmObject = { slug: string; name: string; standard?: boolean; attributes: Attribute[] }

export type StageEdit = {
  object: string
  attribute: string
  action: 'rename' | 'move' | 'delete'
  stage: string
  name?: string
  before?: string
  replacement?: string
}

export type Ref = { id: string; name?: string; photo?: string }

export type Relation = { object: string; attribute: string; limit?: number }

// A value is text, or a referenced record.
export type Value = string | Ref

export type CrmRecord = {
  id: string
  conversation_id?: string
  object: string
  created_at: string
  values: Record<string, Value | Value[] | null>
  related?: Record<string, Ref[]>
  activity?: { interactions: number; first_at?: string; last_at?: string; channel?: string; last_message?: CrmMessage }
  photo?: string
}

export type Kind = 'message' | 'meeting' | 'call' | 'note'

export type Party = { address: string; name?: string; role: string; person_id?: string; photo?: string }

export type RecordRef = { id: string; object: string; name?: string }

export type CrmMessage = { at: string; sender: string; sender_address?: string; recipients?: string[]; direction?: 'sent' | 'received'; text: string; html?: string; partial?: boolean }

export type Speech = { speaker: string; text: string; at?: string }

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
  channel?: string
  author?: string
  text?: string
  invitation?: string
  meet_url?: string
  transcript?: Speech[]
  messages?: CrmMessage[]
  drafts?: { follow_up_id: string; subject: string; state: 'draft' | 'missing' | 'sent' }[]
  provenance?: string
  last_message?: CrmMessage
  drafting?: { state: 'drafting' | 'completed' | 'failed' | 'skipped'; reason?: string; started_at?: string }
}

export type Verdict = 'pending' | 'kept' | 'skipped'

export type TriageRule = { domain: string; decision: 'kept' | 'skipped'; reason: string }

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
  domain?: string
}

export type Member = { name: string; email: string; addresses?: string[]; admin?: boolean; is_me?: boolean; photo?: string }

export type Workspace = { id: string; name: string; description: string; company_page_ids: string[]; drafting_web_access: boolean; timezone: string; members?: Member[]; invited?: string[] }

export type Connection = {
  id: string
  account: string
  owner_id: string
  status: string
  teammates_send: boolean
  created_at: string
  synced: Record<string, string>
  step?: string
  backfilled: boolean
  messages: number
  oldest?: string
}

export type Connections = { connections: Connection[]; connect_url?: string; since: string }

export type DraftMessage = { draft: string; subject: string; to: string[]; cc: string[]; revision?: string }
export type DraftRewriteAction = 'shorten' | 'less_salesy' | 'one_clear_ask' | 'warmer' | 'polish' | 'custom'
export type DraftProposal = Pick<DraftMessage, 'draft' | 'subject'>
export type DraftSender = Omit<DraftMessage, 'draft'> & { from: string; signature?: string; reply: boolean; imported_draft?: boolean; draft?: string; bcc?: string[]; attachments?: string[]; html?: string }

export type TriageSettings = {
  auto_keep_email: boolean
  auto_keep_meetings: boolean
  auto_keep_records: boolean
  auto_keep_ai: boolean
}

export const filterOperators = ['is', 'is_not', 'contains', 'not_contains', 'is_empty', 'is_not_empty', 'before', 'on_or_before', 'after', 'on_or_after'] as const
export type RecordFilter = { attribute: string; operator: (typeof filterOperators)[number]; value?: string }
export type SavedFilter = { id: string; name: string; query?: string; filters: RecordFilter[] }
