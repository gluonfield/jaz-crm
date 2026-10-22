// Package crm embeds the Jaz CRM in a Go program that already signs people
// in and owns a Postgres database.
//
// The host owns the pool: it calls Migrate when it deploys and Open once in
// its composition root, and closes the pool itself. Neither starts servers,
// jobs or provider clients, and Open leaves the schema as it is. On each
// request the host turns the user it signed in into an Actor, and the CRM
// scopes every read and write to that user's tenant.
package crm

import (
	"context"
	"errors"
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

// User is a person the host signed in, in the tenant it admitted them to.
type User struct {
	// Tenant is the host's ID for the company or account whose records the
	// user works on. Everyone in a tenant shares its records; no one else
	// sees them.
	Tenant string
	// ID is the host's stable ID for the person.
	ID   string
	Name string
	// Email marks the user's side of conversations. It is unique within a
	// tenant.
	Email string
}

// hostIssuer names the host's people among identities, where the
// standalone app's sign-in providers use their URLs.
const hostIssuer = "host"

// Actor returns the actor u writes as. The host calls it on every request,
// after it has authenticated u and checked that u belongs to u.Tenant, with
// values from its own records rather than from the request: the CRM trusts
// them. The tenant ID becomes the actor's WorkspaceID. The tenant's
// workspace is created on first use, and the CRM keeps a member for each
// user in each tenant, found by the host's ID and never by email, so
// history names who wrote what.
func (c *CRM) Actor(ctx context.Context, u User) (records.Actor, error) {
	if u.Tenant == "" || u.ID == "" {
		return records.Actor{}, errs.Invalidf("a user needs a tenant and an ID")
	}
	member := storage.NewUser{Name: strings.TrimSpace(u.Name), Email: strings.ToLower(strings.TrimSpace(u.Email))}
	if member.Email == "" {
		return records.Actor{}, errs.Invalidf("a user needs an email")
	}
	current, err := c.store.TenantMember(ctx, u.Tenant, member, storage.Identity{Issuer: hostIssuer, Subject: u.ID}, records.StandardObjects)
	if errors.Is(err, storage.ErrConflict) {
		return records.Actor{}, errs.Invalidf("another user of this tenant has the email %s", member.Email)
	}
	return records.Actor{WorkspaceID: current.WorkspaceID, MemberID: current.ID}, err
}
