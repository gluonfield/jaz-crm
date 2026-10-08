package records

import (
	"cmp"
	"slices"

	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

// FollowUps is the object of commitments: what we or they owe next, by when,
// with an optional draft of the message that moves it on.
const FollowUps = "follow_ups"

const (
	draftAttribute       = "draft"
	draftStatusAttribute = "draft_status"
)

// A follow-up's draft is written, approved by a person, claimed once by
// whoever sends it, and sent. A person can return a claimed draft to
// approved when its sending failed.
const (
	DraftWritten  = "Draft"
	DraftApproved = "Approved"
	DraftSending  = "Sending"
	DraftSent     = "Sent"
)

// draftParts are the attributes that make up a draft; changing one withdraws
// its approval.
var draftParts = []string{draftAttribute, "subject", "channel", "to", "cc"}

// guardDraft applies a follow-up's draft rules to a write against the
// record's current values. Draft status moves by these rules rather than by
// source rank, so an agent can claim a draft a person approved.
func guardDraft(sc schema, object storage.Object, current []storage.RecordValue, set, remove []change, source Source) ([]change, []change, error) {
	if object.Slug != FollowUps {
		return set, remove, nil
	}
	status, err := sc.attribute(object, draftStatusAttribute, false)
	if err != nil {
		return set, remove, nil
	}
	draft, _ := sc.attribute(object, draftAttribute, false)
	now := ""
	written := false
	for _, v := range current {
		switch v.AttributeID {
		case status.ID:
			now = *v.Text
		case draft.ID:
			written = true
		}
	}
	edited := false
	for _, c := range append(slices.Clone(set), remove...) {
		edited = edited || slices.Contains(draftParts, c.attr.Slug)
	}
	if slices.ContainsFunc(set, func(c change) bool { return c.attr.ID == draft.ID }) {
		written = true
	}
	if slices.ContainsFunc(remove, func(c change) bool { return c.attr.ID == draft.ID && len(c.entries) == 0 }) {
		written = false
	}
	i := slices.IndexFunc(set, func(c change) bool { return c.attr.ID == status.ID })
	if i >= 0 {
		to := *set[i].entries[0].text
		// Confirmation saves reviewed recipients and claims the draft together.
		if edited && to != DraftWritten && !(to == DraftSending && source == SourceUser) {
			return nil, nil, errs.Invalidf("change a draft or its status, not both: editing a draft withdraws its approval")
		}
		set[i].force = true
		if err := transition(now, to, source, written); err != nil {
			return nil, nil, err
		}
		if to == DraftSent {
			for _, slug := range []string{draftAttribute, "subject"} {
				attr, _ := sc.attribute(object, slug, false)
				remove = append(remove, change{attr: attr, force: true})
			}
		}
		return set, remove, nil
	}
	if j := slices.IndexFunc(remove, func(c change) bool { return c.attr.ID == status.ID }); j >= 0 {
		remove[j].force = true
		edited = true
	}
	if !edited {
		return set, remove, nil
	}
	if now == DraftSending {
		return nil, nil, errs.Invalidf("the draft is being sent")
	}
	if !written {
		return set, append(remove, change{attr: status, force: true}), nil
	}
	if now != DraftWritten {
		text := DraftWritten
		set = append(set, change{attr: status, entries: []entry{{text: &text}}, force: true})
	}
	return set, remove, nil
}

func transition(from, to string, source Source, written bool) error {
	switch {
	case !written:
		return errs.Invalidf("the follow-up has no draft")
	case to == DraftApproved && source != SourceUser:
		return errs.Invalidf("only a person approves a draft, in the CRM")
	case to == DraftApproved && from == DraftSent:
		return errs.Invalidf("the draft is %s", from)
	case to == DraftSending && from != DraftApproved && !(from == DraftWritten && source == SourceUser):
		return errs.Invalidf("the draft is %s, not %s", cmp.Or(from, "unwritten"), DraftApproved)
	case to == DraftSent && from != DraftSending:
		return errs.Invalidf("the draft is %s, not %s", cmp.Or(from, "unwritten"), DraftSending)
	case to == DraftWritten && from == DraftSending:
		return errs.Invalidf("the draft is being sent")
	}
	return nil
}
