// Package connections owns members' provider accounts: the Google
// authorization flow, encrypted refresh tokens and sync cursors.
package connections

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

var ErrDisabled = errs.Invalid{Message: "Google is not configured: set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET and ENCRYPTION_KEY"}

// Config enables Google connections when OAuth credentials and a 32-byte
// encryption key are set.
type Config struct {
	Google    google.OAuthConfig
	Key       []byte
	Endpoints google.Endpoints
}

// Syncer runs a connection's sync; the Temporal worker implements it.
type Syncer interface {
	Start(ctx context.Context, connectionID string) error
	Stop(ctx context.Context, connectionID string) error
}

type Service struct {
	store  storage.ConnectionStore
	cfg    Config
	aead   cipher.AEAD
	syncer Syncer
}

func NewService(store storage.ConnectionStore, cfg Config, syncer Syncer) (*Service, error) {
	s := &Service{store: store, cfg: cfg, syncer: syncer}
	if len(cfg.Key) == 0 {
		return s, nil
	}
	block, err := aes.NewCipher(cfg.Key)
	if err != nil {
		return nil, errors.New("ENCRYPTION_KEY must be 32 bytes, base64 encoded (openssl rand -base64 32)")
	}
	s.aead, err = cipher.NewGCM(block)
	return s, err
}

func (s *Service) Enabled() bool {
	return s.cfg.Google.ClientID != "" && s.aead != nil
}

// AuthURL starts a person's Google consent.
func (s *Service) AuthURL(state, verifier string) (string, error) {
	if !s.Enabled() {
		return "", ErrDisabled
	}
	return s.cfg.Google.AuthURL(state, verifier), nil
}

// Connect finishes consent: it stores the account for the actor's workspace
// and starts syncing it.
func (s *Service) Connect(ctx context.Context, actor auth.Actor, code, verifier string) (storage.Connection, error) {
	if !s.Enabled() {
		return storage.Connection{}, ErrDisabled
	}
	token, err := s.cfg.Google.Exchange(ctx, code, verifier)
	if err != nil {
		return storage.Connection{}, err
	}
	if token.RefreshToken == "" {
		return storage.Connection{}, errs.Invalidf("Google sent no refresh token; remove the app's access at https://myaccount.google.com/permissions and connect again")
	}
	profile, err := google.NewClient(s.cfg.Google.Client(ctx, token.RefreshToken), s.cfg.Endpoints).Profile(ctx)
	if err != nil {
		return storage.Connection{}, err
	}
	nonce := make([]byte, s.aead.NonceSize())
	_, _ = rand.Read(nonce)
	c, err := s.store.SaveConnection(ctx, storage.NewConnection{
		WorkspaceID: actor.WorkspaceID, UserID: actor.UserID, Provider: "google", Account: profile.Email,
		RefreshToken: s.aead.Seal(nonce, nonce, []byte(token.RefreshToken), nil),
	})
	if err != nil {
		return c, err
	}
	return c, s.syncer.Start(ctx, c.ID)
}

// Google returns an API client for a connection's account.
func (s *Service) Google(ctx context.Context, c storage.Connection) (*google.Client, error) {
	if s.aead == nil {
		return nil, ErrDisabled
	}
	size := s.aead.NonceSize()
	if len(c.RefreshToken) < size {
		return nil, errors.New("stored refresh token is malformed")
	}
	token, err := s.aead.Open(nil, c.RefreshToken[:size], c.RefreshToken[size:], nil)
	if err != nil {
		return nil, err
	}
	return google.NewClient(s.cfg.Google.Client(ctx, string(token)), s.cfg.Endpoints), nil
}

func (s *Service) Connection(ctx context.Context, id string) (storage.Connection, error) {
	return s.store.Connection(ctx, id)
}

func (s *Service) Active(ctx context.Context) ([]storage.Connection, error) {
	return s.store.ActiveConnections(ctx)
}

// Revoke records that the account withdrew access; syncing stops until the
// member connects again.
func (s *Service) Revoke(ctx context.Context, id string) error {
	return s.store.SetConnectionStatus(ctx, id, "revoked")
}

// View is a connection with its sync progress.
type View struct {
	ID        string
	Account   string
	Owner     string
	Status    string
	CreatedAt time.Time
	// Streams maps each stream to when it last moved.
	Streams map[string]time.Time
}

func (s *Service) List(ctx context.Context, actor auth.Actor) ([]View, error) {
	list, err := s.store.Connections(ctx, actor.WorkspaceID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(list))
	for i, c := range list {
		ids[i] = c.ID
	}
	cursors, err := s.store.Cursors(ctx, ids)
	out := []View{}
	for _, c := range list {
		v := View{ID: c.ID, Account: c.Account, Owner: c.UserID, Status: c.Status, CreatedAt: c.CreatedAt, Streams: map[string]time.Time{}}
		for _, cur := range cursors {
			if cur.ConnectionID == c.ID {
				v.Streams[cur.Stream] = cur.UpdatedAt
			}
		}
		out = append(out, v)
	}
	return out, err
}

// Disconnect stops a connection's sync and deletes it; what it already
// synced stays.
func (s *Service) Disconnect(ctx context.Context, actor auth.Actor, id string) error {
	list, err := s.store.Connections(ctx, actor.WorkspaceID)
	if err != nil {
		return err
	}
	for _, c := range list {
		if c.ID == id {
			if err := s.syncer.Stop(ctx, id); err != nil {
				return err
			}
			return s.store.DeleteConnection(ctx, actor.WorkspaceID, id)
		}
	}
	return errs.Invalidf("no connection %q", id)
}

// Cursor returns where a stream resumes, or "" before it starts.
func (s *Service) Cursor(ctx context.Context, connectionID, stream string) (string, error) {
	cursor, err := s.store.Cursor(ctx, connectionID, stream)
	if errors.Is(err, storage.ErrNotFound) {
		return "", nil
	}
	return cursor, err
}

func (s *Service) SetCursor(ctx context.Context, connectionID, stream, cursor string) error {
	return s.store.SetCursor(ctx, connectionID, stream, cursor)
}

func (s *Service) ResetCursor(ctx context.Context, connectionID, stream string) error {
	return s.store.DeleteCursor(ctx, connectionID, stream)
}

// Channel is a Calendar push channel; Google echoes its token on every
// notification.
type Channel struct {
	ID       string    `json:"id"`
	Resource string    `json:"resource"`
	Token    string    `json:"token"`
	Expires  time.Time `json:"expires"`
}

const channelStream = "calendar_channel"

// Channel returns a connection's Calendar channel, zero when it has none.
func (s *Service) Channel(ctx context.Context, connectionID string) (Channel, error) {
	var c Channel
	raw, err := s.Cursor(ctx, connectionID, channelStream)
	if err != nil || raw == "" {
		return c, err
	}
	return c, json.Unmarshal([]byte(raw), &c)
}

func (s *Service) SetChannel(ctx context.Context, connectionID string, c Channel) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return s.SetCursor(ctx, connectionID, channelStream, string(raw))
}
