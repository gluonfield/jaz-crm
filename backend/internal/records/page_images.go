package records

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image/png"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func (s *Service) pageImage(ctx context.Context, workspaceID, value string) (entry, error) {
	var image storage.PageImage
	if token, ok := strings.CutPrefix(value, "image:"); ok {
		var err error
		image, err = s.store.PageImage(ctx, token[strings.LastIndex(token, "/")+1:])
		if errors.Is(err, storage.ErrNotFound) || err == nil && image.WorkspaceID != workspaceID {
			return entry{}, errs.Invalidf("icon: no image in this workspace")
		}
		if err != nil {
			return entry{}, err
		}
	} else {
		encoded, ok := strings.CutPrefix(value, "data:image/png;base64,")
		if !ok || len(encoded) > 100_000 {
			return entry{}, errs.Invalidf("icon: upload a PNG thumbnail up to 128 × 128 pixels")
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return entry{}, errs.Invalidf("icon: invalid PNG image")
		}
		config, err := png.DecodeConfig(bytes.NewReader(decoded))
		if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 128 || config.Height > 128 {
			return entry{}, errs.Invalidf("icon: upload a PNG thumbnail up to 128 × 128 pixels")
		}
		pixels, err := png.Decode(bytes.NewReader(decoded))
		if err != nil {
			return entry{}, errs.Invalidf("icon: invalid PNG image")
		}
		var clean bytes.Buffer
		if err := png.Encode(&clean, pixels); err != nil {
			return entry{}, err
		}
		image.PNG = clean.Bytes()
	}
	text := "image:" + image.ID
	return entry{text: &text, image: &image}, nil
}

func (s *Service) PageImage(ctx context.Context, id string) ([]byte, error) {
	image, err := s.store.PageImage(ctx, id)
	return image.PNG, err
}
