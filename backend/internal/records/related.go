package records

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func (s *Service) include(ctx context.Context, actor auth.Actor, sc schema, target storage.Object, records []Record, relations []Relation) error {
	if len(relations) > 8 {
		return errs.Invalidf("at most 8 included relationships")
	}
	ids := make([]string, len(records))
	for i, record := range records {
		ids[i] = record.ID
	}
	for _, relation := range relations {
		object, err := sc.object(relation.Object)
		if err != nil {
			return err
		}
		attr, err := sc.attribute(object, relation.Attribute)
		if err != nil {
			return err
		}
		if attr.Type != Reference || *attr.TargetObjectID != target.ID {
			return errs.Invalidf("%s.%s must reference %s", relation.Object, relation.Attribute, target.Slug)
		}
		if relation.Limit < 0 || relation.Limit > 20 {
			return errs.Invalidf("included relationship limit must be 1 to 20, default 4")
		}
		if len(ids) == 0 {
			continue
		}
		limit := relation.Limit
		if limit == 0 {
			limit = 4
		}
		found, err := s.store.RelatedRecords(ctx, actor.WorkspaceID, attr.ID, ids, int32(limit))
		if err != nil {
			return err
		}
		children := make([]string, len(found))
		for i, record := range found {
			children[i] = record.ID
		}
		names, err := s.names(ctx, actor.WorkspaceID, sc, children)
		if err != nil {
			return err
		}
		grouped := map[string][]Value{}
		for _, record := range found {
			grouped[record.ParentID] = append(grouped[record.ParentID], Value{RecordID: record.ID, Text: names[record.ID]})
		}
		for i := range records {
			if records[i].Related == nil {
				records[i].Related = map[string][]Value{}
			}
			records[i].Related[relation.Object+"."+relation.Attribute] = grouped[records[i].ID]
		}
	}
	return nil
}

// Referenced fills a record's Related with what references it, such as a
// person's deals and follow-ups, up to limit records per relationship.
func (s *Service) Referenced(ctx context.Context, actor auth.Actor, record Record, limit int) (Record, error) {
	sc, err := s.schema(ctx, actor.WorkspaceID)
	if err != nil {
		return record, err
	}
	target, err := sc.object(record.Object)
	if err != nil {
		return record, err
	}
	var relations []Relation
	for _, attr := range sc.attrs {
		if attr.Type == Reference && *attr.TargetObjectID == target.ID && len(relations) < 8 {
			relations = append(relations, Relation{Object: sc.objectByID(attr.ObjectID).Slug, Attribute: attr.Slug, Limit: limit})
		}
	}
	records := []Record{record}
	err = s.include(ctx, actor, sc, target, records, relations)
	return records[0], err
}
