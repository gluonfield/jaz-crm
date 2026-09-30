package records

import (
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
	Checkbox  = "checkbox"
	URL       = "url"
	Select    = "select"
	Email     = "email"
	Domain    = "domain"
	Phone     = "phone"
	Reference = "reference"
)

// uniqueTypes can identify a record.
var uniqueTypes = []string{Text, Number, URL, Email, Domain, Phone}

// titleAttribute names a record wherever it is referenced.
const titleAttribute = "name"

// StandardObjects is the schema every workspace starts with.
var StandardObjects = []storage.NewObject{
	{Slug: "people", Name: "People", Attributes: []storage.NewAttribute{
		{Slug: titleAttribute, Name: "Name", Type: Text},
		{Slug: "email_addresses", Name: "Email addresses", Type: Email, Multi: true, IsUnique: true},
		{Slug: "phone_numbers", Name: "Phone numbers", Type: Phone, Multi: true, IsUnique: true},
		{Slug: "company", Name: "Company", Type: Reference, Target: "companies"},
		{Slug: "job_title", Name: "Job title", Type: Text},
	}},
	{Slug: "companies", Name: "Companies", Attributes: []storage.NewAttribute{
		{Slug: titleAttribute, Name: "Name", Type: Text},
		{Slug: "domains", Name: "Domains", Type: Domain, Multi: true, IsUnique: true},
		{Slug: "description", Name: "Description", Type: Text},
	}},
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
		return strconv.FormatFloat(n, 'f', -1, 64), err == nil
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
	case Select:
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
