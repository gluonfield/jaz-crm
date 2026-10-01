package workspaces_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/gluonfield/jaz-tasks/auth"
)

func TestFirebaseMigrationPreservesGoogleMemberships(t *testing.T) {
	ctx := context.Background()
	store := postgrestest.New(t)
	people := workspaces.NewService(store, workspaces.Config{})
	google := signin.Identity{Issuer: "https://accounts.google.com", Subject: "google-ada", Email: "ada@example.com", EmailVerified: true, Name: "Ada"}
	original, err := people.SignIn(ctx, google)
	if err != nil {
		t.Fatal(err)
	}
	other, err := people.SignIn(ctx, signin.Identity{Issuer: "https://accounts.google.com", Subject: "google-bob", Email: "bob@example.com", EmailVerified: true, Name: "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = people.Invite(ctx, auth.Actor{UserID: other.ID, WorkspaceID: other.WorkspaceID}, google.Email)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := people.SignIn(ctx, google); err != nil {
		t.Fatal(err)
	}
	firebase := signin.Identity{
		Issuer: "https://securetoken.google.com/shared-pool", Subject: "firebase-ada", Email: "renamed@example.com", EmailVerified: true,
		LinkedIdentities: []signin.Subject{{Issuer: google.Issuer, Subject: google.Subject}},
	}
	unverified := firebase
	unverified.EmailVerified = false
	if _, err := people.SignIn(ctx, unverified); !errors.Is(err, workspaces.ErrEmailUnverified) {
		t.Fatalf("unverified migration: %v", err)
	}
	linked, err := store.UsersByIdentity(ctx, firebase.Issuer, firebase.Subject)
	if err != nil || len(linked) != 0 {
		t.Fatalf("unverified identity was linked: %v, %v", linked, err)
	}
	migrated, err := people.SignIn(ctx, firebase)
	if err != nil || migrated.ID != original.ID || migrated.WorkspaceID != original.WorkspaceID {
		t.Fatalf("membership changed: %+v -> %+v, %v", original, migrated, err)
	}
	linked, err = store.UsersByIdentity(ctx, firebase.Issuer, firebase.Subject)
	if err != nil || len(linked) != 2 {
		t.Fatalf("all memberships must survive: %v, %v", linked, err)
	}
	firebase.LinkedIdentities = nil
	again, err := people.SignIn(ctx, firebase)
	if err != nil || again.ID != original.ID {
		t.Fatalf("returning Firebase identity: %+v, %v", again, err)
	}
	unrelated, err := people.SignIn(ctx, signin.Identity{Issuer: firebase.Issuer, Subject: "unrelated-uid", Email: google.Email, EmailVerified: true})
	if err != nil || unrelated.WorkspaceID == original.WorkspaceID || unrelated.WorkspaceID == other.WorkspaceID {
		t.Fatalf("email alone linked an unrelated account: %+v, %v", unrelated, err)
	}
}
