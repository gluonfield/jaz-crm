// Package logos finds the icons companies publish on their websites and
// keeps them, so the CRM shows a company's logo without the app reaching
// out to other sites.
package logos

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/google/uuid"
)

type Service struct {
	store storage.LogoStore
	fetch Fetcher
}

func NewService(store storage.LogoStore, fetch Fetcher) *Service {
	return &Service{store: store, fetch: fetch}
}

// refreshBatch is how many domains one refresh looks up.
const refreshBatch = 20

// Refresh looks up the logos of a workspace's domains that are due.
func (s *Service) Refresh(ctx context.Context, workspaceID string) error {
	domains, err := s.store.StaleLogoDomains(ctx, workspaceID, refreshBatch)
	if err != nil {
		return err
	}
	for _, domain := range domains {
		logo := s.fetch.Icon(ctx, domain)
		// A cancelled look found nothing about the site.
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.store.SaveLogo(ctx, logo); err != nil {
			return err
		}
	}
	return nil
}

// Logo returns the image a token names.
func (s *Service) Logo(ctx context.Context, token string) (storage.Logo, error) {
	if uuid.Validate(token) != nil {
		return storage.Logo{}, storage.ErrNotFound
	}
	return s.store.LogoByToken(ctx, token)
}

// Tokens maps the workspace's records that have a logo to its token.
func (s *Service) Tokens(ctx context.Context, actor auth.Actor, recordIDs []string) (map[string]string, error) {
	return s.store.RecordLogos(ctx, actor.WorkspaceID, recordIDs)
}
