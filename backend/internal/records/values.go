package records

import (
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

// Attribute types.
const (
	Text      = "text"
	Email     = "email"
	Domain    = "domain"
	Phone     = "phone"
	Reference = "reference"
)

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

// InvalidInputError is a caller's mistake, safe to show as is.
type InvalidInputError struct {
	Message string
}

func (e InvalidInputError) Error() string {
	return e.Message
}

func invalid(format string, args ...any) error {
	return InvalidInputError{Message: fmt.Sprintf(format, args...)}
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
		return entry{}, invalid("%s: empty value", attr.Slug)
	}
	text, key := raw, strings.ToLower(raw)
	switch attr.Type {
	case Email:
		addr, err := mail.ParseAddress(raw)
		if err != nil {
			return entry{}, invalid("%s: %q is not an email address", attr.Slug, raw)
		}
		text = strings.ToLower(addr.Address)
		key = text
	case Domain:
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
		if !strings.Contains(host, ".") || strings.ContainsAny(host, " @") {
			return entry{}, invalid("%s: %q is not a domain", attr.Slug, raw)
		}
		text = host
		key = host
	case Phone:
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, raw)
		if len(digits) < 6 {
			return entry{}, invalid("%s: %q is not a phone number", attr.Slug, raw)
		}
		key = digits
		if strings.HasPrefix(raw, "+") {
			key = "+" + digits
		}
	}
	e := entry{text: &text}
	if attr.IsUnique {
		e.key = &key
	}
	return e, nil
}
