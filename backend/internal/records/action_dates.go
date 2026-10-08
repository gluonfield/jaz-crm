package records

import (
	"slices"

	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

var actionDateParts = []string{"action_date", "action_date_basis", "action_date_reason", "action_date_source"}

func guardFollowUp(sc schema, object storage.Object, current []storage.RecordValue, set, remove []change, source Source) ([]change, []change, []Skip, error) {
	if object.Slug != FollowUps {
		return set, remove, nil, nil
	}
	// Derive effective fields through the same source precedence as the write.
	proposed, _ := plan(current, set, remove, source, "")
	next := func(slug string) string {
		attr, _ := sc.attribute(object, slug, false)
		for _, v := range proposed.Insert {
			if v.AttributeID == attr.ID && v.Text != nil {
				return *v.Text
			}
		}
		for _, v := range current {
			if v.AttributeID == attr.ID && v.Text != nil && !slices.Contains(proposed.Close, v.ID) {
				return *v.Text
			}
		}
		return ""
	}
	held := func(slug string) *storage.RecordValue {
		attr, _ := sc.attribute(object, slug, false)
		for i := range current {
			if current[i].AttributeID == attr.ID {
				return &current[i]
			}
		}
		return nil
	}
	drop := func(slugs []string) {
		set = slices.DeleteFunc(set, func(c change) bool { return slices.Contains(slugs, c.attr.Slug) })
		remove = slices.DeleteFunc(remove, func(c change) bool { return slices.Contains(slugs, c.attr.Slug) })
	}
	clear := func(slugs []string) {
		drop(slugs)
		for _, slug := range slugs {
			attr, _ := sc.attribute(object, slug, false)
			remove = append(remove, change{attr: attr, force: true})
		}
	}
	var skipped []Skip
	closed := next("status") == "Done" || next("status") == "Dismissed"
	manual := held("action_date_basis")
	date := held("action_date")
	dateChanged := slices.ContainsFunc(set, func(c change) bool { return c.attr.Slug == "action_date" }) || slices.ContainsFunc(remove, func(c change) bool { return c.attr.Slug == "action_date" })
	switch {
	case closed:
		clear(actionDateParts)
	case source != SourceUser && (manual != nil && *manual.Text == "Manual" || date != nil && date.Source == string(SourceUser)):
		for _, c := range append(slices.Clone(set), remove...) {
			if slices.Contains(actionDateParts, c.attr.Slug) {
				skipped = append(skipped, Skip{Attribute: c.attr.Slug, Source: SourceUser})
			}
		}
		drop(actionDateParts)
	case dateChanged && source == SourceUser:
		clear(actionDateParts[1:])
		remove = slices.DeleteFunc(remove, func(c change) bool { return c.attr.Slug == "action_date_basis" })
		attr, _ := sc.attribute(object, "action_date_basis", false)
		text := "Manual"
		set = append(set, change{attr: attr, entries: []entry{{text: &text}}, force: true})
	case dateChanged && next("action_date") == "":
		clear(actionDateParts)
	case dateChanged:
		for _, slug := range []string{"action_date_basis", "action_date_reason"} {
			if !slices.ContainsFunc(set, func(c change) bool { return c.attr.Slug == slug }) {
				return nil, nil, nil, errs.Invalidf("an action date change needs %s", slug)
			}
		}
		if !slices.ContainsFunc(set, func(c change) bool { return c.attr.Slug == "action_date_source" }) {
			clear([]string{"action_date_source"})
		}
		if next("action_date_basis") != "Stated" && next("action_date_basis") != "Suggested" {
			return nil, nil, nil, errs.Invalidf("an action date needs a Stated or Suggested basis")
		}
		if next("action_date_reason") == "" {
			return nil, nil, nil, errs.Invalidf("an action date needs its evidence or scheduling reason")
		}
	}
	if closed || next("waiting_on") == "Them" {
		if source == SourceAgent {
			drop(draftParts)
		}
		if draft := held(draftAttribute); draft != nil && draft.Source == string(SourceAgent) && (held(draftStatusAttribute) == nil || *held(draftStatusAttribute).Text != DraftSending) && !slices.ContainsFunc(set, func(c change) bool { return c.attr.Slug == draftAttribute }) {
			clear([]string{draftAttribute})
		}
	}
	set, remove, err := guardDraft(sc, object, current, set, remove, source)
	return set, remove, skipped, err
}
