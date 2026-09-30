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
}

type NewAttribute struct {
	Slug     string
	Name     string
	Type     string
	Multi    bool
	IsUnique bool
	Target   string
}

type RecordQuery struct {
	WorkspaceID string
	ObjectID    string
	Query       *string
	// AttributeIDs and Matches pair up: each record must hold a current value
	// of the attribute whose lowercased text, unique key or reference equals
	// the match.
	AttributeIDs []string
	Matches      []string
	Limit        int32
}

// ValueChanges closes current values by ID and inserts new ones.
type ValueChanges struct {
	Close  []int64
	Insert []NewRecordValue
}

// RecordMutation decides a record's changes from its current values.
type RecordMutation func(current []RecordValue) (ValueChanges, error)

type RecordStore interface {
	Objects(ctx context.Context, workspaceID string) ([]Object, error)
	Attributes(ctx context.Context, workspaceID string) ([]Attribute, error)
	Records(ctx context.Context, workspaceID string, ids []string) ([]Record, error)
	SearchRecords(ctx context.Context, query RecordQuery) ([]Record, error)
	CurrentValues(ctx context.Context, workspaceID string, recordIDs []string) ([]RecordValue, error)
	// RecordsByUniqueKeys returns the records holding any (attribute, key) pair.
	RecordsByUniqueKeys(ctx context.Context, workspaceID string, attributeIDs, keys []string) ([]string, error)
	// WriteRecord locks the record, creating it when id is empty, and applies
	// mutate's changes in one transaction, returning the record's id.
	WriteRecord(ctx context.Context, workspaceID, objectID, id string, mutate RecordMutation) (string, error)
}
