package postgres

import (
	"context"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	logodb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/logos"
)

func (s *Store) StaleLogoDomains(ctx context.Context, workspaceID string, limit int32) ([]string, error) {
	domains, err := s.logo.StaleLogoDomains(ctx, logodb.StaleLogoDomainsParams{WorkspaceID: workspaceID, Limit: limit})
	return domains, mapError(err)
}

func (s *Store) SaveLogo(ctx context.Context, logo storage.Logo) error {
	return mapError(s.logo.SaveLogo(ctx, logodb.SaveLogoParams(logo)))
}

func (s *Store) LogoByToken(ctx context.Context, token string) (storage.Logo, error) {
	row, err := s.logo.LogoByToken(ctx, token)
	return storage.Logo{ContentType: row.ContentType, Image: row.Image}, mapError(err)
}

func (s *Store) RecordLogos(ctx context.Context, workspaceID string, recordIDs []string) (map[string]string, error) {
	rows, err := s.logo.RecordLogos(ctx, logodb.RecordLogosParams{WorkspaceID: workspaceID, RecordIDs: recordIDs})
	tokens := map[string]string{}
	for _, r := range rows {
		tokens[r.RecordID] = r.Token
	}
	return tokens, mapError(err)
}
