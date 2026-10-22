// Package crm embeds the Jaz CRM in a Go program that already signs people
// in and owns a Postgres database.
//
// The host owns the pool: it calls Migrate when it deploys and Open once in
// its composition root, and closes the pool itself. Neither starts servers,
// jobs or provider clients, and Open leaves the schema as it is. The host
// authenticates people and decides who belongs to which workspace; the CRM
// maps each host person to a member of a workspace and scopes every read and
// write to the workspace of the Actor it is given.
package crm

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/gluonfield/jaz-crm/errs"
	"github.com/gluonfield/jaz-crm/interactions"
	"github.com/gluonfield/jaz-crm/internal/storage"
	"github.com/gluonfield/jaz-crm/internal/storage/postgres"
	"github.com/gluonfield/jaz-crm/records"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Options struct {
	// Schema holds the CRM's tables and its migration history apart from
	// the host's. Empty is the standalone app's layout: both live wherever
	// the connections' search_path points, usually public.
	Schema string
	// Classifier triages incoming conversations; without one they wait for
	// a person.
	Classifier interactions.Classifier
}

type CRM struct {
	Records      *records.Service
	Interactions *interactions.Service
	store        *postgres.Store
}

// Migrate brings schema to the latest version, creating it when missing.
func Migrate(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	return postgres.Migrate(ctx, pool, schema)
}

// Open builds the CRM on pool. It fails while schema has pending migrations.
func Open(ctx context.Context, pool *pgxpool.Pool, opts Options) (*CRM, error) {
	store, err := postgres.New(ctx, pool, opts.Schema)
	if err != nil {
		return nil, err
	}
	recs := records.NewService(store)
	return &CRM{
		Records:      recs,
		Interactions: interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: recs, Classifier: opts.Classifier}),
		store:        store,
	}, nil
}

// Principal is a person as the host knows them: Issuer names the host and
// Subject is its stable ID for the person. The CRM keeps it beside the
// member it maps to and never reads it as a member ID or an email.
type Principal struct {
	Issuer  string
	Subject string
}

// Member is a person joining a workspace. The email marks their own side of
// conversations and must be unique within the workspace.
type Member struct {
	Principal
	Name  string
	Email string
}

var ErrNotMember = errs.Invalid{Message: "not a member of this workspace"}

// CreateWorkspace starts a workspace with the standard objects and owner as
// its admin, and returns the owner's actor there.
func (c *CRM) CreateWorkspace(ctx context.Context, name string, owner Member) (records.Actor, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return records.Actor{}, errs.Invalidf("a workspace needs a name")
	}
	user, err := owner.parse()
	if err != nil {
		return records.Actor{}, err
	}
	user.Admin = true
	created, err := c.store.CreateOwnedWorkspace(ctx, name, user, owner.identity(), records.StandardObjects)
	return actor(created), err
}

// AddMember makes m a member of the workspace, or returns their actor there
// when they already are one.
func (c *CRM) AddMember(ctx context.Context, workspaceID string, m Member) (records.Actor, error) {
	if existing, err := c.ActAs(ctx, m.Principal, workspaceID); !errors.Is(err, ErrNotMember) {
		return existing, err
	}
	user, err := m.parse()
	if err != nil {
		return records.Actor{}, err
	}
	user.WorkspaceID = workspaceID
	added, err := c.store.AddMember(ctx, user, m.identity())
	if errors.Is(err, storage.ErrConflict) {
		// A concurrent call may have added the same person first.
		if existing, err := c.ActAs(ctx, m.Principal, workspaceID); err == nil {
			return existing, nil
		}
		return records.Actor{}, errs.Invalidf("another member of this workspace has the email %s", user.Email)
	}
	return actor(added), err
}

// ActAs returns the actor of p's member in the workspace, or ErrNotMember.
func (c *CRM) ActAs(ctx context.Context, p Principal, workspaceID string) (records.Actor, error) {
	if err := p.check(); err != nil {
		return records.Actor{}, err
	}
	users, err := c.store.UsersByIdentity(ctx, p.Issuer, p.Subject)
	if err != nil {
		return records.Actor{}, err
	}
	i := slices.IndexFunc(users, func(u storage.User) bool { return u.WorkspaceID == workspaceID })
	if i < 0 {
		return records.Actor{}, ErrNotMember
	}
	return actor(users[i]), nil
}

// check keeps host people apart from the standalone app's own: the "email"
// issuer stands for whoever proves an address, which no host may claim.
func (p Principal) check() error {
	switch {
	case p.Issuer == "" || p.Subject == "":
		return errs.Invalidf("a principal needs an issuer and a subject")
	case p.Issuer == "email":
		return errs.Invalidf(`the issuer "email" is reserved`)
	}
	return nil
}

func (p Principal) identity() storage.Identity {
	return storage.Identity{Issuer: p.Issuer, Subject: p.Subject}
}

func (m Member) parse() (storage.NewUser, error) {
	user := storage.NewUser{Name: strings.TrimSpace(m.Name), Email: strings.ToLower(strings.TrimSpace(m.Email))}
	if user.Email == "" {
		return user, errs.Invalidf("a member needs an email")
	}
	return user, m.check()
}

func actor(u storage.User) records.Actor {
	return records.Actor{WorkspaceID: u.WorkspaceID, MemberID: u.ID}
}
