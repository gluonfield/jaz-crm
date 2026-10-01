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
	"slices"
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
	// Backfill is how far back a connection syncs mail and meetings.
	Backfill time.Duration
}

// Streams a connection syncs, each resuming from its cursor.
const (
	StreamBackfill = "gmail_backfill"
	StreamHistory  = "gmail_history"
	StreamWatch    = "gmail_watch"
	StreamCalendar = "calendar"
	// StreamPhotos holds when profile pictures were last read.
	StreamPhotos = "photos"
	// BackfillDone is the backfill's cursor once the mail history is in.
	BackfillDone = "done"
)

// Syncer runs a connection's sync; the Temporal worker implements it.
type Syncer interface {
	Start(ctx context.Context, connectionID string) error
	Stop(ctx context.Context, connectionID string) error
	// Step names the sync step running now, or "" between passes.
	Step(ctx context.Context, connectionID string) (string, error)
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

// Since is where the synced window starts.
func (s *Service) Since() time.Time {
	return time.Now().Add(-s.cfg.Backfill)
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
	ID        string    `json:"id"`
	Account   string    `json:"account"`
	Owner     string    `json:"owner_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	// TeammatesSend lets other members send replies from this mailbox.
	TeammatesSend bool `json:"teammates_send"`
	// Streams maps each stream to when it last moved.
	Streams map[string]time.Time `json:"synced"`
	// Step is the sync step running now, such as GmailBackfill; empty
	// between passes.
	Step string `json:"step,omitempty"`
	// Backfilled reports whether the mail history is in.
	Backfilled bool `json:"backfilled"`
	// Messages counts the synced mail; Oldest is the earliest.
	Messages int        `json:"messages"`
	Oldest   *time.Time `json:"oldest,omitempty"`
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
	if err != nil {
		return nil, err
	}
	mail, err := s.store.MailProgress(ctx, ids)
	out := []View{}
	for _, c := range list {
		v := View{ID: c.ID, Account: c.Account, Owner: c.UserID, Status: c.Status, CreatedAt: c.CreatedAt, TeammatesSend: c.TeammatesSend, Streams: map[string]time.Time{}}
		for _, cur := range cursors {
			if cur.ConnectionID == c.ID {
				v.Streams[cur.Stream] = cur.UpdatedAt
				v.Backfilled = v.Backfilled || cur.Stream == StreamBackfill && cur.Cursor == BackfillDone
			}
		}
		for _, m := range mail {
			if m.ConnectionID == c.ID {
				v.Messages, v.Oldest = m.Messages, &m.Oldest
			}
		}
		if c.Status == "active" {
			v.Step = s.step(ctx, c.ID)
		}
		out = append(out, v)
	}
	return out, err
}

// step asks the syncer what runs now. Progress is advisory: an unreachable
// Temporal reports idle rather than failing the list.
func (s *Service) step(ctx context.Context, connectionID string) string {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	step, _ := s.syncer.Step(ctx, connectionID)
	return step
}

// Mailbox is the connection the actor sends from: the preferred one when it
// is active and theirs or open to teammates, else the actor's own.
func (s *Service) Mailbox(ctx context.Context, actor auth.Actor, preferred string) (storage.Connection, error) {
	list, err := s.store.Connections(ctx, actor.WorkspaceID)
	if err != nil {
		return storage.Connection{}, err
	}
	usable := func(c storage.Connection) bool {
		return c.Status == "active" && (c.UserID == actor.UserID || c.TeammatesSend)
	}
	if i := slices.IndexFunc(list, func(c storage.Connection) bool { return c.ID == preferred }); i >= 0 && usable(list[i]) {
		return list[i], nil
	}
	if i := slices.IndexFunc(list, func(c storage.Connection) bool { return c.UserID == actor.UserID && usable(c) }); i >= 0 {
		return list[i], nil
	}
	return storage.Connection{}, errs.Invalidf("no mailbox to send from: connect your Google account")
}

// SetTeammatesSend decides whether the workspace's other members may send
// replies from the actor's own mailbox.
func (s *Service) SetTeammatesSend(ctx context.Context, actor auth.Actor, id string, allowed bool) error {
	err := s.store.SetTeammatesSend(ctx, actor.WorkspaceID, actor.UserID, id, allowed)
	if errors.Is(err, storage.ErrNotFound) {
		return errs.Invalidf("no connection %q of yours", id)
	}
	return err
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

// AddAliases records other addresses the connection's mailbox sends as or
// receives at.
func (s *Service) AddAliases(ctx context.Context, connectionID string, aliases []string) error {
	return s.store.AddAliases(ctx, connectionID, aliases)
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
