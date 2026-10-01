package interactions

import (
	"context"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
)

type DomainRule struct {
	Domain   string `json:"domain"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

func (s *Service) DomainRules(ctx context.Context, actor auth.Actor) ([]DomainRule, error) {
	rules, err := s.store.DomainRules(ctx, actor.WorkspaceID)
	out := []DomainRule{}
	for _, rule := range rules {
		out = append(out, DomainRule{Domain: rule.Domain, Decision: rule.Triage, Reason: rule.Reason})
	}
	return out, err
}

func (s *Service) ForgetDomainRule(ctx context.Context, actor auth.Actor, domain string) error {
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), "@")
	return s.store.DeleteDomainRule(ctx, actor.WorkspaceID, domain)
}
