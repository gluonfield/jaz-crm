package auth

import (
	"context"
	"errors"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

const SessionTTL = 30 * 24 * time.Hour

// CreateSession starts a browser session and returns its cookie value.
func (s *Service) CreateSession(ctx context.Context, userID string) (string, time.Time, error) {
	token := secret("")
	expires := s.now().Add(SessionTTL)
	return token, expires, s.store.CreateSession(ctx, hash(token), userID, expires)
}

func (s *Service) Session(ctx context.Context, token string) (Actor, error) {
	user, err := s.store.UserBySession(ctx, hash(token))
	if errors.Is(err, storage.ErrNotFound) {
		return Actor{}, ErrUnauthenticated
	}
	actor := actorOf(user)
	actor.session = hash(token)
	return actor, err
}

func (s *Service) EndSession(ctx context.Context, token string) error {
	return s.store.DeleteSession(ctx, hash(token))
}

// Describe names who an actor is and which workspace they act in.
func (s *Service) Describe(ctx context.Context, actor Actor) (storage.User, storage.Workspace, error) {
	user, err := s.store.UserByID(ctx, actor.UserID)
	if err != nil {
		return user, storage.Workspace{}, err
	}
	workspace, err := s.store.Workspace(ctx, actor.WorkspaceID)
	return user, workspace, err
}
