package workspaces_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/gluonfield/jaz-tasks/auth"
)

func TestMembersSetTheirOwnAddressesAndAdminsAnyones(t *testing.T) {
	ctx := context.Background()
	store := postgrestest.New(t)
	people := workspaces.NewService(store, workspaces.Config{})
	owner, err := people.SignIn(ctx, signin.Identity{Issuer: "test", Subject: "owner", Email: "owner@cas.dev", EmailVerified: true, Name: "Owner"})
	if err != nil {
		t.Fatal(err)
	}
	admin := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	if _, err := people.Invite(ctx, admin, "pat@cas.dev"); err != nil {
		t.Fatal(err)
	}
	pat, err := people.SignIn(ctx, signin.Identity{Issuer: "test", Subject: "pat", Email: "pat@cas.dev", EmailVerified: true, Name: "Pat"})
	if err != nil || pat.WorkspaceID != owner.WorkspaceID || pat.Admin {
		t.Fatalf("pat joins as a member: %+v %v", pat, err)
	}
	member := auth.Actor{UserID: pat.ID, WorkspaceID: pat.WorkspaceID}
	outsider, err := people.Provision(ctx, "eve@else.dev")
	if err != nil {
		t.Fatal(err)
	}

	user, err := people.SetAddresses(ctx, member, "PAT@cas.dev", []string{" Pat@Uni.ac.uk ", "pat@uni.ac.uk", "pat@cas.dev", "p.home@gmail.com"})
	if want := []string{"pat@uni.ac.uk", "p.home@gmail.com"}; err != nil || !slices.Equal(user.Addresses, want) {
		t.Fatalf("own addresses: %v %v, want %v", user.Addresses, err, want)
	}
	if _, err := people.SetAddresses(ctx, member, "owner@cas.dev", []string{"owner@uni.ac.uk"}); !errors.Is(err, workspaces.ErrForbidden) {
		t.Fatalf("a member changed another's addresses: %v", err)
	}
	if _, err := people.SetAddresses(ctx, auth.Actor{UserID: outsider.ID, WorkspaceID: outsider.WorkspaceID}, "pat@cas.dev", nil); err == nil {
		t.Fatal("another workspace changed a member's addresses")
	}
	if _, err := people.SetAddresses(ctx, member, "pat@cas.dev", []string{"not an address"}); err == nil {
		t.Fatal("accepted an invalid address")
	}
	if _, err := people.SetAddresses(ctx, admin, "pat@cas.dev", []string{"pat@uni.ac.uk"}); err != nil {
		t.Fatalf("admin changes a member's addresses: %v", err)
	}
	users, _, err := people.Members(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(users, func(u storage.User) bool { return u.ID == pat.ID })
	if i < 0 || !slices.Equal(users[i].Addresses, []string{"pat@uni.ac.uk"}) {
		t.Fatalf("stored addresses: %+v", users)
	}
}
