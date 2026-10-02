package storage

import (
	"context"
	"time"
)

type Object struct {
	ID          string
	WorkspaceID string
	Slug        string
	Name        string
	CreatedAt   time.Time
}

type Attribute struct {
	ID             string
	ObjectID       string
	Slug           string
	Name           string
	Type           string
	Multi          bool
	IsUnique       bool
	TargetObjectID *string
	CreatedAt      time.Time
	Options        []string
}

// AttributeInput creates one attribute on an existing object.
type AttributeInput struct {
	ObjectID       string
	Slug           string
	Name           string
	Type           string
	Multi          bool
	IsUnique       bool
	TargetObjectID *string
	Options        []string
}

type Record struct {
	ID          string
	WorkspaceID string
	ObjectID    string
	CreatedAt   time.Time
}

// RecordValue is one value of a record's attribute. A current value has no
// ActiveUntil; closed values are its history.
type RecordValue struct {
	ID          int64
	RecordID    string
	AttributeID string
	Text        *string
	RefRecordID *string
	UniqueKey   *string
	Source      string
	ActorID     *string
	ActiveFrom  time.Time
	ActiveUntil *time.Time
}

// PastValue is a value from a record's history with who set it.
type PastValue struct {
	RecordValue
	ActorName string
}

type NewRecordValue struct {
	RecordID    string
	AttributeID string
	Text        *string
	RefRecordID *string
	UniqueKey   *string
	Source      string
	ActorID     *string
}

// NewObject declares an object with its attributes; a reference attribute
// names its target object by slug.
type NewObject struct {
	Slug       string
	Name       string
	Attributes []NewAttribute
	Filters    []SavedFilter
}

type NewAttribute struct {
	Slug     string
	Name     string
	Type     string
	Multi    bool
	IsUnique bool
	Target   string
	Options  []string
}

type RecordQuery struct {
	WorkspaceID  string
	ObjectID     string
	Query        *string
	AttributeIDs []string
	Operators    []string
	Matches      []string
	// SortAttributeID orders records by that date attribute, earliest first
	// and undated last; nil keeps the newest first.
	SortAttributeID *string
	Limit           int32
}

type RelatedRecord struct {
	ParentID string
	ID       string
}

type RecordFilter struct {
	Attribute string `json:"attribute"`
	Operator  string `json:"operator" jsonschema:"is, is_not, contains, not_contains, is_empty, is_not_empty, before, on_or_before, after or on_or_after; date comparisons require a date field; all conditions must match"`
	Value     string `json:"value,omitempty" jsonschema:"a matching value; date filters also accept today, evaluated in UTC when searched"`
}

type SavedFilter struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Query    string         `json:"query,omitempty"`
	Filters  []RecordFilter `json:"filters"`
	ObjectID string         `json:"-"`
}

// ValueChanges closes current values by ID and inserts new ones. Revise
// rewrites current values' text in place, keeping one version of a document
// while one writer goes on editing it.
type ValueChanges struct {
	Close  []int64
	Insert []NewRecordValue
	Revise []ValueRevision
}

type ValueRevision struct {
	ID   int64
	Text string
}

// RecordMutation decides a record's changes from its current values.
type RecordMutation func(current []RecordValue) (ValueChanges, error)

type StatusChanges struct {
	Options []string
	From    string
	To      string
	ActorID *string
}

type StatusMutation func(options []string) (StatusChanges, error)

type RecordStore interface {
	Objects(ctx context.Context, workspaceID string) ([]Object, error)
	Attributes(ctx context.Context, workspaceID string) ([]Attribute, error)
	CreateObject(ctx context.Context, workspaceID string, object NewObject) error
	CreateAttribute(ctx context.Context, attr AttributeInput) error
	RenameObject(ctx context.Context, workspaceID, id, name string) error
	// DeleteObject removes an object with its records, their values and the
	// reference attributes pointing at it, with those attributes' conditions
	// in saved filters.
	DeleteObject(ctx context.Context, workspaceID, id string) error
	RenameAttribute(ctx context.Context, workspaceID, id, name string) error
	// DeleteAttribute removes an attribute with its values and its conditions
	// in saved filters.
	DeleteAttribute(ctx context.Context, workspaceID string, attr Attribute) error
	AddAttributeOption(ctx context.Context, workspaceID, attributeID, value string) (string, error)
	EditStatus(ctx context.Context, workspaceID, attributeID string, mutate StatusMutation) error
	DeleteRecord(ctx context.Context, workspaceID, id string) error
	Records(ctx context.Context, workspaceID string, ids []string) ([]Record, error)
	SearchRecords(ctx context.Context, query RecordQuery) ([]Record, error)
	RelatedRecords(ctx context.Context, workspaceID, attributeID string, ids []string, limit int32) ([]RelatedRecord, error)
	SavedFilters(ctx context.Context, workspaceID, objectID string) ([]SavedFilter, error)
	SaveFilter(ctx context.Context, workspaceID string, filter SavedFilter) (SavedFilter, error)
	DeleteFilter(ctx context.Context, workspaceID, id string) error
	CurrentValues(ctx context.Context, workspaceID string, recordIDs []string) ([]RecordValue, error)
	// History returns a record's values, current and closed, newest first.
	History(ctx context.Context, workspaceID, recordID string, limit int32) ([]PastValue, error)
	// RecordsByUniqueKeys returns the records holding any (attribute, key) pair.
	RecordsByUniqueKeys(ctx context.Context, workspaceID string, attributeIDs, keys []string) ([]string, error)
	// WriteRecord locks the record, creating it when id is empty, and applies
	// mutate's changes in one transaction, returning the record's id.
	WriteRecord(ctx context.Context, workspaceID, objectID, id string, mutate RecordMutation) (string, error)
	// Users lists a workspace's members, whom member attributes name.
	Users(ctx context.Context, workspaceID string) ([]User, error)
}
