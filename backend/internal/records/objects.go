package records

import (
	"slices"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

// Channels name where messages are exchanged. A conversation's channel is a
// name in lower case; a follow-up's channel is the name.
var Channels = []string{"Email", "LinkedIn", "WhatsApp", "X", "Telegram", "SMS"}

// ChannelName names a conversation channel, empty when it is none.
func ChannelName(channel string) string {
	i := slices.IndexFunc(Channels, func(name string) bool { return strings.ToLower(name) == channel })
	if i < 0 {
		return ""
	}
	return Channels[i]
}

// StandardObjects is the schema every workspace starts with, listed in this
// order.
var StandardObjects = []storage.NewObject{
	{Slug: "companies", Name: "Companies", Attributes: []storage.NewAttribute{
		{Slug: titleAttribute, Name: "Name", Type: Text},
		{Slug: "categories", Name: "Categories", Type: Select, Multi: true},
		{Slug: "domains", Name: "Domains", Type: Domain, Multi: true, IsUnique: true},
		{Slug: "description", Name: "Description", Type: Text},
		{Slug: "website", Name: "Website", Type: URL},
		{Slug: LinksAttribute, Name: "Links", Type: URL, Multi: true, IsUnique: true},
		{Slug: "industry", Name: "Industry", Type: Select},
		{Slug: "hq_city", Name: "Headquarters city", Type: Text},
		{Slug: "hq_state", Name: "Headquarters state / region", Type: Text},
		{Slug: "hq_country", Name: "Headquarters country", Type: Select},
		{Slug: "employee_count", Name: "Employee count", Type: Number},
		{Slug: "owner", Name: "Owner", Type: Member},
		{Slug: "notes", Name: "Notes", Type: Text},
		{Slug: "founded_year", Name: "Founded year", Type: Number},
		{Slug: "size", Name: "Size", Type: Select, Options: []string{"1-10", "11-50", "51-200", "201-500", "501-1,000", "1,001-5,000", "5,001-10,000", "10,001+"}},
	}},
	{Slug: "people", Name: "People", Attributes: []storage.NewAttribute{
		{Slug: titleAttribute, Name: "Name", Type: Text},
		{Slug: "tags", Name: "Tags", Type: Select, Multi: true},
		{Slug: "email_addresses", Name: "Email addresses", Type: Email, Multi: true, IsUnique: true},
		{Slug: "phone_numbers", Name: "Phone numbers", Type: Phone, Multi: true, IsUnique: true},
		{Slug: "company", Name: "Company", Type: Reference, Target: "companies"},
		{Slug: "job_title", Name: "Job title", Type: Text},
		{Slug: LinksAttribute, Name: "Links", Type: URL, Multi: true, IsUnique: true},
		{Slug: "owner", Name: "Owner", Type: Member},
		{Slug: "notes", Name: "Notes", Type: Text},
		{Slug: ContextAttribute, Name: "Context", Type: Text},
	}},
	{Slug: "deals", Name: "Deals", Attributes: []storage.NewAttribute{
		{Slug: titleAttribute, Name: "Name", Type: Text},
		{Slug: "stage", Name: "Stage", Type: Status, Options: []string{"Lead", "In progress", "On hold", "Won", "Lost"}},
		{Slug: "owner", Name: "Owner", Type: Member},
		{Slug: "next_follow_up_date", Name: "Next follow-up date", Type: Date},
		{Slug: "next_action", Name: "Next action", Type: Text},
		{Slug: "value", Name: "Value", Type: Number},
		{Slug: "expected_close_date", Name: "Expected close date", Type: Date},
		{Slug: "company", Name: "Company", Type: Reference, Target: "companies"},
		{Slug: "people", Name: "People", Type: Reference, Target: "people", Multi: true},
	}, Filters: []storage.SavedFilter{{Name: "Due follow-ups", Filters: []storage.RecordFilter{
		{Attribute: "next_follow_up_date", Operator: "on_or_before", Value: "today"},
		{Attribute: "stage", Operator: "is_not", Value: "Won"},
		{Attribute: "stage", Operator: "is_not", Value: "Lost"},
	}}}},
	{Slug: FollowUps, Name: "Follow-ups", Attributes: []storage.NewAttribute{
		{Slug: titleAttribute, Name: "Action", Type: Text},
		{Slug: "status", Name: "Status", Type: Status, Options: []string{"Open", "Done", "Dismissed"}},
		{Slug: "waiting_on", Name: "Waiting on", Type: Select, Options: []string{"Us", "Them"}},
		{Slug: "action_date", Name: "Action date", Type: DateTime},
		{Slug: "action_date_basis", Name: "Date basis", Type: Select, Options: []string{"Stated", "Suggested", "Manual"}},
		{Slug: "action_date_reason", Name: "Date reason", Type: Text},
		{Slug: "action_date_source", Name: "Date source", Type: Text},
		{Slug: "owner", Name: "Owner", Type: Member},
		{Slug: "person", Name: "Person", Type: Reference, Target: "people"},
		{Slug: "company", Name: "Company", Type: Reference, Target: "companies"},
		{Slug: "deal", Name: "Deal", Type: Reference, Target: "deals"},
		{Slug: "subject", Name: "Subject", Type: Text},
		{Slug: draftAttribute, Name: "Draft", Type: Text},
		{Slug: "channel", Name: "Channel", Type: Select, Options: Channels},
		{Slug: "to", Name: "To", Type: Email, Multi: true},
		{Slug: "cc", Name: "Cc", Type: Email, Multi: true},
		{Slug: draftStatusAttribute, Name: "Draft status", Type: Select, Options: []string{DraftWritten, DraftApproved, DraftSending, DraftSent}},
	}, Filters: []storage.SavedFilter{
		{Name: "Needs attention", Filters: []storage.RecordFilter{
			{Attribute: "status", Operator: "is", Value: "Open"},
			{Attribute: "waiting_on", Operator: "is_not", Value: "Them"},
		}},
		{Name: "Chase", Filters: []storage.RecordFilter{
			{Attribute: "status", Operator: "is", Value: "Open"},
			{Attribute: "waiting_on", Operator: "is", Value: "Them"},
			{Attribute: "action_date", Operator: "on_or_before", Value: "now"},
		}},
		{Name: "Waiting on them", Filters: []storage.RecordFilter{
			{Attribute: "status", Operator: "is", Value: "Open"},
			{Attribute: "waiting_on", Operator: "is", Value: "Them"},
		}},
	}},
	{Slug: Pages, Name: "Pages", Attributes: []storage.NewAttribute{
		{Slug: titleAttribute, Name: "Name", Type: Text},
		{Slug: "parent", Name: "Parent", Type: Reference, Target: Pages},
		{Slug: ContentAttribute, Name: "Content", Type: Markdown},
		{Slug: "icon", Name: "Icon", Type: Text},
	}},
}

// standard reports whether every workspace has the object, or the attribute
// of an object when one is named. The CRM relies on these; they stay.
func standard(object string, attribute ...string) bool {
	i := slices.IndexFunc(StandardObjects, func(o storage.NewObject) bool { return o.Slug == object })
	return i >= 0 && (len(attribute) == 0 || slices.ContainsFunc(StandardObjects[i].Attributes, func(a storage.NewAttribute) bool { return a.Slug == attribute[0] }))
}
