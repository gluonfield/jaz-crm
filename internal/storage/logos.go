package storage

import "context"

// Logo is a domain's website icon; a nil Image means none was found.
type Logo struct {
	Domain      string
	ContentType string
	Image       []byte
}

type LogoStore interface {
	// StaleLogoDomains are a workspace's record domains whose logo is due a
	// look.
	StaleLogoDomains(ctx context.Context, workspaceID string, limit int32) ([]string, error)
	SaveLogo(ctx context.Context, logo Logo) error
	// LogoByToken returns ErrNotFound for a token naming no image.
	LogoByToken(ctx context.Context, token string) (Logo, error)
	// RecordLogos maps records to the token of their domain's logo.
	RecordLogos(ctx context.Context, workspaceID string, recordIDs []string) (map[string]string, error)
}
