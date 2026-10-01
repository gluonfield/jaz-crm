package interactions

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

// Known is the workspace's own addresses and company domains.
type Known struct {
	WorkspaceID string
	own         map[string]bool
	domains     map[string]bool
}

func (s *Service) Known(ctx context.Context, workspaceID string) (Known, error) {
	k := Known{WorkspaceID: workspaceID, own: map[string]bool{}, domains: map[string]bool{}}
	addresses, err := s.conns.InternalAddresses(ctx, workspaceID)
	if err != nil {
		return k, err
	}
	for _, address := range addresses {
		k.own[address] = true
		if domain := workDomain(address); domain != "" {
			k.domains[domain] = true
		}
	}
	return k, nil
}

var automated = regexp.MustCompile(`^([^@]*[+._-])?(no-?reply|do-?not-?reply|notifications?|mailer-daemon|postmaster|bounces?)([+._-][^@]*)?@|@(resource\.)?calendar\.google\.com$`)

// verdict is a new address's first verdict.
func (k Known) verdict(kind, value string) storage.NewHandle {
	h := storage.NewHandle{WorkspaceID: k.WorkspaceID, Kind: kind, Value: value, Triage: Pending}
	if kind != "email" {
		return h
	}
	domain := domainOf(value)
	switch {
	case k.own[value] || k.domains[domain]:
		h.Triage = Internal
	case automated.MatchString(value):
		h.Triage, h.DecidedBy, h.Reason = Skipped, ptr(ByRule), "automated sender"
	}
	return h
}

var noiseLabels = []string{"CATEGORY_PROMOTIONS", "CATEGORY_SOCIAL", "CATEGORY_UPDATES", "CATEGORY_FORUMS"}

// noise reports mail Gmail filed as promotions, social, updates or forums,
// unless someone marked it important or starred it.
func noise(labels []string) bool {
	if slices.Contains(labels, "IMPORTANT") || slices.Contains(labels, "STARRED") {
		return false
	}
	return slices.ContainsFunc(labels, func(l string) bool { return slices.Contains(noiseLabels, l) })
}

var freemailDomains = []string{
	"gmail.com", "googlemail.com", "outlook.com", "hotmail.com", "live.com", "msn.com", "yahoo.com", "ymail.com",
	"icloud.com", "me.com", "mac.com", "aol.com", "proton.me", "protonmail.com", "gmx.com", "gmx.de", "web.de",
	"mail.com", "yandex.com", "yandex.ru", "fastmail.com", "hey.com", "zoho.com", "qq.com", "163.com",
}

// workDomain is an email address's company domain, or empty for webmail and
// for anything that is not an email address.
func workDomain(address string) string {
	if domain := domainOf(address); !slices.Contains(freemailDomains, domain) {
		return domain
	}
	return ""
}

func domainOf(address string) string {
	_, domain, _ := strings.Cut(address, "@")
	return domain
}

// companyName guesses a company's name from its domain: acme-tools.com is Acme-tools.
func companyName(domain string) string {
	label, _, _ := strings.Cut(domain, ".")
	runes := []rune(label)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}

var replyPrefix = regexp.MustCompile(`(?i)^\s*((re|fwd?|aw|sv|wg)\s*(\[\d+\])?\s*:\s*)+`)

// subject is a thread's title without reply and forward prefixes.
func subject(raw string) string {
	return strings.TrimSpace(replyPrefix.ReplaceAllString(raw, ""))
}

func ptr[T any](v T) *T {
	return &v
}
