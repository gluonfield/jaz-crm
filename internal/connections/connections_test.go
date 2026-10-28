package connections_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gluonfield/jaz-crm/internal/auth"
	"github.com/gluonfield/jaz-crm/internal/connections"
	"github.com/gluonfield/jaz-crm/internal/storage"
	"github.com/gluonfield/jaz-crm/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/internal/workspaces"
	"github.com/gluonfield/jaz-crm/records"
	"github.com/gluonfield/jaz-tasks/auth"
)

type syncRecorder struct{ stops int }

func (*syncRecorder) Start(context.Context, string) error { return nil }
func (s *syncRecorder) Stop(context.Context, string) error {
	s.stops++
	return nil
}
func (*syncRecorder) Step(context.Context, string) (string, error) { return "", nil }

func TestDisconnectRequiresMailboxOwner(t *testing.T) {
	ctx := t.Context()
	store := postgrestest.New(t)
	people := workspaces.NewService(store, workspaces.Config{})
	owner, err := people.Provision(ctx, "owner@audit.test")
	if err != nil {
		t.Fatal(err)
	}
	admin := auth.Actor{Actor: records.Actor{MemberID: owner.ID, WorkspaceID: owner.WorkspaceID}}
	if _, err := people.Invite(ctx, admin, "member@audit.test"); err != nil {
		t.Fatal(err)
	}
	member, err := people.SignIn(ctx, signin.Identity{Issuer: "audit", Subject: "member", Email: "member@audit.test", EmailVerified: true})
	if err != nil || member.Admin || member.WorkspaceID != owner.WorkspaceID {
		t.Fatalf("invalid non-admin membership: %+v %v", member, err)
	}
	connection, err := store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: owner.WorkspaceID, UserID: owner.ID, Provider: "google", Account: owner.Email, RefreshToken: []byte("test-provider-boundary")})
	if err != nil {
		t.Fatal(err)
	}
	syncer := &syncRecorder{}
	svc, err := connections.NewService(store, connections.Config{}, syncer)
	if err != nil {
		t.Fatal(err)
	}
	peer := records.Actor{MemberID: member.ID, WorkspaceID: member.WorkspaceID}
	if err := svc.Disconnect(ctx, peer, connection.ID); err == nil {
		t.Fatal("member disconnected another member's mailbox")
	}
	if syncer.stops != 0 {
		t.Fatal("denied disconnect stopped sync")
	}
	if err := store.DeleteConnection(ctx, peer.WorkspaceID, peer.MemberID, connection.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("storage accepted non-owner deletion: %v", err)
	}
	if _, err := store.Connection(ctx, connection.ID); err != nil {
		t.Fatalf("denied disconnect removed connection: %v", err)
	}
	if err := svc.Disconnect(ctx, admin.Actor, connection.ID); err != nil {
		t.Fatal(err)
	}
	_, err = store.Connection(ctx, connection.ID)
	if !errors.Is(err, storage.ErrNotFound) || syncer.stops != 1 {
		t.Fatalf("owner disconnect failed: %v, stops=%d", err, syncer.stops)
	}
}
