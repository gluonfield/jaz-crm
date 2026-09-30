package mcpapi

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/logos"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
)

// pictures finds the pictures of records and of the records they reference:
// people's profile pictures, and the logos of records with a domain, which
// the CRM serves itself.
type pictures struct {
	people    *interactions.Service
	logos     *logos.Service
	publicURL string
}

func (p pictures) of(ctx context.Context, actor auth.Actor, shown []records.Record) (map[string]string, error) {
	var ids []string
	for _, r := range shown {
		ids = append(ids, r.ID)
		for _, f := range r.Fields {
			for _, v := range f.Values {
				if v.RecordID != "" {
					ids = append(ids, v.RecordID)
				}
			}
		}
	}
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
