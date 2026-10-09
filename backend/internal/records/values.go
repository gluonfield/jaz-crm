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

// LinksAttribute holds the pages a person or company is on, such as their
// LinkedIn and X profiles; like an email address, a link identifies them.
const LinksAttribute = "links"

// ContentAttribute is the markdown body of a page and of every record of a
// workspace's own tables. A link to /r/<record id> mentions that record.
const ContentAttribute = "content"

// Pages are documents that nest under a parent page.
const Pages = "pages"

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
		switch attr.Type {
		case Phone:
			key = phoneKey(raw)
		case URL:
			key = LinkKey(text)
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
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return "", false
		}
		if attr.Slug == "founded_year" && (n < 1 || n > 9999 || math.Trunc(n) != n) {
			return "", false
		}
		return strconv.FormatFloat(n, 'f', -1, 64), true
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

// LinkKey is what makes two links to one page the same: host and path in
// lower case, without scheme, www, query, fragment or trailing slash, with
// LinkedIn's country hosts and twitter.com folded into linkedin.com and
// x.com. It is empty for anything without a web host.
func LinkKey(raw string) string {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.Contains(u.Hostname(), ".") {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	switch {
	case strings.HasSuffix(host, ".linkedin.com"):
		host = "linkedin.com"
	case host == "twitter.com" || host == "mobile.twitter.com":
		host = "x.com"
	}
	return host + strings.ToLower(strings.TrimRight(u.Path, "/"))
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
