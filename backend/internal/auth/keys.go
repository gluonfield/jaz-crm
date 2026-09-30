package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

// CreateKey stores a personal API key for the user, generating one when key
// is empty, and returns the key; only its hash is kept.
func (s *Service) CreateKey(ctx context.Context, userID, label, key string) (string, storage.APIKey, error) {
	if label = strings.TrimSpace(label); label == "" {
		return "", storage.APIKey{}, fmt.Errorf("a label is required")
	}
	if key == "" {
		key = secret(apiKeyPrefix)
	}
	record, err := s.store.CreateAPIKey(ctx, userID, label, hint(key), hash(key))
	return key, record, err
}

// ProvisionKey makes a key supplied from outside the user's only key with the
// label, so replacing it retires the previous one.
func (s *Service) ProvisionKey(ctx context.Context, userID, label, key string) error {
	if len(key) < 32 || strings.HasPrefix(key, accessTokenPrefix) {
		return errors.New("the key must be at least 32 random characters, such as the output of openssl rand -hex 32")
	}
	_, err := s.store.ReplaceAPIKey(ctx, userID, label, hint(key), hash(key))
	if errors.Is(err, storage.ErrNotFound) {
		return errors.New("the key already belongs to another account")
	}
	return err
}

func hint(key string) string {
	return "…" + key[max(0, len(key)-4):]
}

func (s *Service) APIKeys(ctx context.Context, actor Actor) ([]storage.APIKey, error) {
	return s.store.APIKeys(ctx, actor.UserID)
}

func (s *Service) DeleteKey(ctx context.Context, actor Actor, id string) error {
	return s.store.DeleteAPIKey(ctx, actor.UserID, id)
}

// CreateKeyForEmail mints a key for the active user with that email. A person
// in several workspaces needs the workspace, by name or ID.
func (s *Service) CreateKeyForEmail(ctx context.Context, email, workspace string) (string, error) {
	users, err := s.store.UsersByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	var match []storage.User
	for _, user := range users {
		if workspace != "" && user.WorkspaceID != workspace {
			ws, err := s.store.Workspace(ctx, user.WorkspaceID)
			if err != nil {
				return "", err
			}
			if !strings.EqualFold(ws.Name, workspace) {
				continue
			}
		}
		match = append(match, user)
	}
	if len(match) != 1 {
		return "", fmt.Errorf("%d active users have email %q in the workspaces asked for; name one workspace", len(match), email)
	}
	key, _, err := s.CreateKey(ctx, match[0].ID, "Minted from the command line", "")
	return key, err
}
