package records

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func (s *Service) pageImage(ctx context.Context, workspaceID, value string) (entry, error) {
	if strings.HasPrefix(value, "data:") {
		return entry{}, errs.Invalidf("icon: use an image URL; file uploads are unavailable")
	}
	token := strings.TrimPrefix(value, "image:")
	if link, err := url.Parse(token); err == nil && (link.Scheme == "https" || link.Scheme == "http") && link.Hostname() != "" && link.User == nil {
		return entry{text: &value}, nil
	}
	image, err := s.store.PageImage(ctx, token)
	if errors.Is(err, storage.ErrNotFound) || err == nil && image.WorkspaceID != workspaceID {
		return entry{}, errs.Invalidf("icon: enter an HTTP or HTTPS image URL")
	}
	return entry{text: &value}, err
}

func (s *Service) PageImage(ctx context.Context, id string) ([]byte, error) {
	image, err := s.store.PageImage(ctx, id)
	return image.PNG, err
}
