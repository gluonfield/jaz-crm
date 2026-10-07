package records

import (
	"context"
	"slices"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
)

// Change is one edit in a record's history: a value set, or one removed with
// nothing set in its place. Actor names the member behind a person's or
// their agent's edit; removals do not record one.
type Change struct {
	Attribute string
	Value     Value
	Removed   bool
	Source    Source
	Actor     string
	At        time.Time
}

// historyLimit bounds how many values one history reads.
const historyLimit = 200

// History lists a record's recent changes, newest first.
func (s *Service) History(ctx context.Context, actor auth.Actor, id string) ([]Change, error) {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return nil, err
	}
	found, err := s.store.Records(ctx, actor.WorkspaceID, []string{id})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, errs.Invalidf("no record %q", id)
	}
	past, err := s.store.History(ctx, actor.WorkspaceID, id, historyLimit)
	if err != nil {
		return nil, err
	}
	var refs []string
	// A value closed in the write that set its successor was replaced, not
	// removed.
	replaced := map[string]bool{}
	for _, v := range past {
		if v.RefRecordID != nil {
			refs = append(refs, *v.RefRecordID)
		}
		replaced[v.AttributeID+v.ActiveFrom.String()] = true
	}
	labels, err := s.labels(ctx, actor.WorkspaceID, sc, refs)
	if err != nil {
		return nil, err
	}
	changes := []Change{}
	for _, v := range past {
		c := Change{Attribute: sc.attributeByID(v.AttributeID).Slug, Source: Source(v.Source), Actor: v.ActorName, At: v.ActiveFrom}
		if v.RefRecordID != nil {
			c.Value = labels[*v.RefRecordID]
		} else {
			c.Value = sc.textValue(sc.attributeByID(v.AttributeID), *v.Text)
		}
		changes = append(changes, c)
		if v.ActiveUntil != nil && !replaced[v.AttributeID+v.ActiveUntil.String()] {
			changes = append(changes, Change{Attribute: c.Attribute, Value: c.Value, Removed: true, At: *v.ActiveUntil})
		}
	}
	slices.SortStableFunc(changes, func(a, b Change) int { return b.At.Compare(a.At) })
	return changes, nil
}
