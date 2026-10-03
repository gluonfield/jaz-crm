package records

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/google/uuid"
)

type Filter = storage.RecordFilter

type Search struct {
	Object  string
	Query   string
	Where   map[string]string
	Filters []Filter
	// Sort names a date or text attribute to order by, earliest or first
	// alphabetically, empty last; empty lists the newest first. Offset skips
	// records for the next page.
	Sort                string
	Offset              int
	Limit               int
	Include             []Relation
	GroupByConversation bool
	ConversationID      string
}

type Relation struct {
	Object    string `json:"object"`
	Attribute string `json:"attribute" jsonschema:"reference attribute pointing to the searched object"`
	Limit     int    `json:"limit,omitempty" jsonschema:"per searched record, at most 20, default 4"`
}

// Search lists a page of an object's matching records and how many match.
func (s *Service) Search(ctx context.Context, actor auth.Actor, q Search) ([]Record, int, error) {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return nil, 0, err
	}
	object, err := sc.object(q.Object)
	if err != nil {
		return nil, 0, err
	}
	filters := append([]Filter{}, q.Filters...)
	for slug, value := range q.Where {
		filters = append(filters, Filter{Attribute: slug, Operator: "is", Value: value})
	}
	query, err := s.filterQuery(ctx, actor, sc, object, filters)
	if err != nil {
		return nil, 0, err
	}
	if (q.GroupByConversation || q.ConversationID != "") && object.Slug != FollowUps {
		return nil, 0, errs.Invalidf("conversation searches require follow_ups")
	}
	query.GroupByConversation = q.GroupByConversation
	if id := strings.TrimSpace(q.ConversationID); id != "" {
		if uuid.Validate(id) != nil {
			return nil, 0, errs.Invalidf("invalid conversation_id")
		}
		query.ConversationID = &id
	}
	if q.Sort != "" {
		attr, err := sc.attribute(object, q.Sort)
		if err != nil {
			return nil, 0, err
		}
		if attr.Type != Date && attr.Type != DateTime && attr.Type != Text {
			return nil, 0, errs.Invalidf("sort takes a date or text attribute; %s is %s", attr.Slug, attr.Type)
		}
		query.SortAttributeID = &attr.ID
	}
	query.Offset = int32(max(q.Offset, 0))
	query.Limit = 20
	if q.Limit > 0 {
		query.Limit = int32(min(q.Limit, 500))
	}
	if text := strings.TrimSpace(q.Query); text != "" {
		escaped := literalPattern(text)
		query.Query = &escaped
	}
	found, total, err := s.store.SearchRecords(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	stored := make([]storage.Record, len(found))
	for i, record := range found {
		stored[i] = record.Record
	}
	views, err := s.views(ctx, actor.WorkspaceID, sc, stored, false)
	if err != nil {
		return nil, 0, err
	}
	for i := range views {
		views[i].ConversationID = found[i].ConversationID
	}
	if err := s.include(ctx, actor, sc, object, views, q.Include); err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

func (s *Service) filterQuery(ctx context.Context, actor auth.Actor, sc schema, object storage.Object, filters []Filter) (storage.RecordQuery, error) {
	query := storage.RecordQuery{WorkspaceID: actor.WorkspaceID, ObjectID: object.ID, AttributeIDs: []string{}, Matches: []string{}, Operators: []string{}}
	if len(filters) > 32 {
		return query, errs.Invalidf("at most 32 filter conditions")
	}
	for _, f := range filters {
		attr, err := sc.attribute(object, f.Attribute)
		if err != nil {
			return query, err
		}
		match := ""
		if attr.Type == Date && f.Value == "today" {
			f.Value = time.Now().UTC().Format(time.DateOnly)
		}
		switch f.Operator {
		case "is_empty", "is_not_empty":
		case "before", "on_or_before", "after", "on_or_after":
			if attr.Type != Date && attr.Type != DateTime {
				return query, errs.Invalidf("%s does not support %s", attr.Slug, f.Operator)
			}
			if attr.Type == DateTime && (f.Value == "today" || f.Value == "now") {
				match = f.Value
				break
			}
			e, err := normalize(attr, f.Value)
			if err != nil {
				return query, err
			}
			match = e.match()
		case "contains", "not_contains":
			if attr.Type == Reference || attr.Type == Number || attr.Type == Date || attr.Type == DateTime || attr.Type == Checkbox {
				return query, errs.Invalidf("%s does not support %s", attr.Slug, f.Operator)
			}
			if match = strings.TrimSpace(f.Value); match == "" {
				return query, errs.Invalidf("%s: empty filter value", attr.Slug)
			}
			match = literalPattern(match)
		case "is", "is_not":
			if attr.Type == DateTime && (f.Value == "today" || f.Value == "now") {
				match = f.Value
				break
			}
			e, err := s.entry(ctx, actor.WorkspaceID, sc, attr, f.Value)
			var unknown errs.Invalid
			if attr.Type == Reference && errors.As(err, &unknown) {
				match = "unresolved"
			} else if err != nil {
				return query, err
			} else {
				match = e.match()
			}
		default:
			return query, errs.Invalidf("unknown filter operator %q", f.Operator)
		}
		query.AttributeIDs = append(query.AttributeIDs, attr.ID)
		query.Matches = append(query.Matches, match)
		query.Operators = append(query.Operators, f.Operator)
	}
	return query, nil
}

func literalPattern(text string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(text)
}
