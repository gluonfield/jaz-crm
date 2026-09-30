// Package auth owns identity: sessions for people, OAuth 2.1 grants for
// agents and apps, and personal API keys for scripts.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

var ErrUnauthenticated = errors.New("authentication required: sign in, or send an OAuth access token or API key in the Authorization header")

// Actor is the authenticated user and the workspace every request is scoped to.
type Actor struct {
	UserID      string
	WorkspaceID string
	// Agent is set for bearer credentials: the request comes from software
	// acting for the user rather than from the person's own browser.
	Agent bool
	// The session or OAuth grant the request came with, which a switch of
	// workspace moves; an API key has neither and stays in its workspace.
	session []byte
	grant   string
}

// Principal names who holds the credential and stays the same when it
// switches workspace: the OAuth grant, or the user for an API key.
func (a Actor) Principal() string {
	if a.grant != "" {
		return "grant:" + a.grant
	}
	return a.UserID
}

// ErrFixedWorkspace is a switch asked of an API key, which belongs to one
// workspace's user.
var ErrFixedWorkspace = errs.Invalid{Message: "an API key acts in its own workspace; create one in the other workspace"}

// Switch points the session or OAuth grant behind actor at another of the
// person's users, so its later requests act in that user's workspace.
func (s *Service) Switch(ctx context.Context, actor Actor, userID string) error {
	switch {
	case actor.grant != "":
		return s.store.UpdateOAuthGrantUser(ctx, actor.grant, userID)
	case actor.session != nil:
		return s.store.UpdateSessionUser(ctx, actor.session, userID)
	}
	return ErrFixedWorkspace
}

func actorOf(user storage.User) Actor {
	return Actor{UserID: user.ID, WorkspaceID: user.WorkspaceID}
}

type Config struct {
	// PublicURL is the issuer of OAuth tokens and the base of every auth URL.
	PublicURL string
}

type Service struct {
	store storage.AuthStore
	cfg   Config
	now   func() time.Time
}

func NewService(store storage.AuthStore, cfg Config) *Service {
	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")
	return &Service{store: store, cfg: cfg, now: time.Now}
}

const (
	apiKeyPrefix       = "jc_api_"
	accessTokenPrefix  = "jc_at_"
	refreshTokenPrefix = "jc_rt_"
)

// Authenticate resolves a bearer token: an OAuth access token or an API key.
func (s *Service) Authenticate(ctx context.Context, token string) (Actor, error) {
	var user storage.User
	var grant string
	var err error
	switch {
	case strings.HasPrefix(token, accessTokenPrefix):
		user, grant, err = s.store.UserByAccessToken(ctx, hash(token))
	case token != "":
		user, err = s.store.UserByAPIKey(ctx, hash(token))
	default:
		return Actor{}, ErrUnauthenticated
	}
	if errors.Is(err, storage.ErrNotFound) {
		return Actor{}, ErrUnauthenticated
	}
	actor := actorOf(user)
	actor.Agent, actor.grant = true, grant
	return actor, err
}

func secret(prefix string) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return prefix + base64.RawURLEncoding.EncodeToString(b)
}

func hash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

type actorKey struct{}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

func ActorFrom(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorKey{}).(Actor)
	return actor, ok
}
