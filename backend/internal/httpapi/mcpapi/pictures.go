package mcpapi

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/logos"
)

// pictures finds records' pictures: people's profile pictures, and the
// logos of records with a domain, which the CRM serves itself.
type pictures struct {
	people    *interactions.Service
	logos     *logos.Service
	publicURL string
}

func (p pictures) of(ctx context.Context, actor auth.Actor, ids []string) (map[string]string, error) {
	found, err := p.people.Photos(ctx, actor, ids)
	if err != nil {
		return nil, err
	}
	tokens, err := p.logos.Tokens(ctx, actor, ids)
	for id, token := range tokens {
		if found[id] == "" {
			found[id] = p.publicURL + "/logos/" + token
		}
	}
	return found, err
}
