package records

import (
	"math"
	"net"
	"net/mail"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

// Attribute types.
const (
	Text      = "text"
	Number    = "number"
	Date      = "date"
	DateTime  = "datetime"
	Checkbox  = "checkbox"
	URL       = "url"
	Select    = "select"
	Status    = "status"
	Member    = "member"
	Email     = "email"
	Domain    = "domain"
	Phone     = "phone"
	Reference = "reference"
	Markdown  = "markdown"
)

// uniqueTypes can identify a record.
var uniqueTypes = []string{Text, Number, URL, Email, Domain, Phone}

// titleAttribute names a record wherever it is referenced.
const titleAttribute = "name"

// ContextAttribute is a person's relationship so far as a short summary.
// Every write replaces it whole, whoever wrote it before: writers merge the
// current context with what they learned, and its history keeps each version.
const ContextAttribute = "context"

// ContentAttribute is the markdown body of a page and of every record of a
// workspace's own tables. A link to /r/<record id> mentions that record.
const ContentAttribute = "content"

// Pages are documents that nest under a parent page.
const Pages = "pages"

// StandardObjects is the schema every workspace starts with, listed in this
// order.
var StandardObjects = []storage.NewObject{
	{Slug: "companies", Name: "Companies", Attributes: []storage.NewAttribute{
		{Slug: titleAttribute, Name: "Name", Type: Text},
		{Slug: "categories", Name: "Categories", Type: Select, Multi: true},
		{Slug: "domains", Name: "Domains", Type: Domain, Multi: true, IsUnique: true},
		{Slug: "description", Name: "Description", Type: Text},
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
		{Slug: ContextAttribute, Name: "Context", Type: Text},
	}},
	{Slug: "deals", Name: "Deals", Attributes: []storage.NewAttribute{
		{Slug: titleAttribute, Name: "Name", Type: Text},
		{Slug: "stage", Name: "Stage", Type: Status, Options: []string{"Lead", "In progress", "On hold", "Won", "Lost"}},
		{Slug: "owner", Name: "Owner", Type: Member},
		{Slug: "next_follow_up_date", Name: "Next follow-up date", Type: Date},
		{Slug: "next_action", Name: "Next action", Type: Text},
		{Slug: "value", Name: "Value", Type: Number},
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
		{Slug: "channel", Name: "Channel", Type: Select, Options: []string{"Email", "LinkedIn"}},
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
	}},
}

// standard reports whether every workspace has the object, or the attribute
// of an object when one is named. The CRM relies on these; they stay.
func standard(object string, attribute ...string) bool {
	i := slices.IndexFunc(StandardObjects, func(o storage.NewObject) bool { return o.Slug == object })
	return i >= 0 && (len(attribute) == 0 || slices.ContainsFunc(StandardObjects[i].Attributes, func(a storage.NewAttribute) bool { return a.Slug == attribute[0] }))
}

// entry is a validated value ready to store: text, or a referenced record,
// with the unique key of a unique attribute.
type entry struct {
	text *string
	ref  *string
	key  *string
}

// identity is what makes two values of one attribute the same value.
func (e entry) identity() string {
	return identity(e.text, e.ref, e.key)
}

func valueIdentity(v storage.RecordValue) string {
	return identity(v.Text, v.RefRecordID, v.UniqueKey)
}

func identity(text, ref, key *string) string {
	switch {
	case ref != nil:
		return *ref
	case key != nil:
		return *key
	}
	return *text
}

// match is what a search filter compares: see storage.RecordQuery.
func (e entry) match() string {
	if e.ref != nil || e.key != nil {
		return e.identity()
	}
	return strings.ToLower(*e.text)
}

// normalize validates a raw value for a non-reference attribute.
func normalize(attr storage.Attribute, raw string) (entry, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return entry{}, errs.Invalidf("%s: empty value", attr.Slug)
	}
	text, ok := canonical(attr, raw)
	if !ok {
		return entry{}, errs.Invalidf("%s: %q is not a valid %s", attr.Slug, raw, attr.Type)
	}
	e := entry{text: &text}
	if attr.IsUnique {
		key := strings.ToLower(text)
		if attr.Type == Phone {
			key = phoneKey(raw)
		}
		e.key = &key
	}
	return e, nil
}

// canonical is the stored form of a value, reporting whether it is valid.
func canonical(attr storage.Attribute, raw string) (string, bool) {
	switch attr.Type {
	case Number:
		n, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64)
		if attr.Slug == "founded_year" && (n < 1 || n > 9999 || math.Trunc(n) != n) {
			return "", false
		}
		return strconv.FormatFloat(n, 'f', -1, 64), err == nil
	case DateTime:
		if d, err := time.Parse(time.DateOnly, raw); err == nil {
			return d.Format(time.DateOnly), d.Year() > 0
		}
		d, err := time.Parse(time.RFC3339Nano, raw)
		return d.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano), err == nil && d.Year() > 0
	case Date:
		for _, layout := range []string{time.DateOnly, time.RFC3339} {
			if d, err := time.Parse(layout, raw); err == nil {
				return d.Format(time.DateOnly), true
			}
		}
		return "", false
	case Checkbox:
		b, err := strconv.ParseBool(strings.ToLower(raw))
		return strconv.FormatBool(b), err == nil
	case URL:
		if !strings.Contains(raw, "://") {
			raw = "https://" + raw
		}
		u, err := url.Parse(raw)
		if err != nil {
			return "", false
		}
		return u.String(), (u.Scheme == "http" || u.Scheme == "https") && strings.Contains(u.Host, ".")
	case Select, Status:
		i := slices.IndexFunc(attr.Options, func(o string) bool { return strings.EqualFold(o, raw) })
		if i < 0 {
			return "", false
		}
		return attr.Options[i], true
	case Email:
		addr, err := mail.ParseAddress(raw)
		if err != nil {
			return "", false
		}
		return strings.ToLower(addr.Address), true
	case Domain:
		return domain(raw)
	case Phone:
		return raw, len(strings.TrimPrefix(phoneKey(raw), "+")) >= 6
	}
	return raw, true
}

// domain reduces a URL or host to its registrable-looking host.
func domain(raw string) (string, bool) {
	host := strings.ToLower(raw)
	if u, err := url.Parse(host); err == nil && u.Host != "" {
		host = u.Host
	} else {
		host, _, _ = strings.Cut(host, "/")
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimPrefix(host, "www.")
	return host, strings.Contains(host, ".") && !strings.ContainsAny(host, " @")
}

func phoneKey(raw string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, raw)
	if strings.HasPrefix(raw, "+") {
		return "+" + digits
	}
	return digits
}
