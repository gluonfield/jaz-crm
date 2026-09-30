// Package workspaces owns membership: who may sign in, the workspace a new
// person starts with, and invites.
package workspaces

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

var (
	ErrEmailUnverified = errors.New("your identity provider has not verified this email address")
	ErrNotAllowed      = errors.New("this email address is not allowed to sign in here")
	ErrForbidden       = errors.New("only workspace admins can invite people or rename the workspace")
	ErrInvalidEmail    = errors.New("enter a valid email address")
	ErrInvalidName     = errors.New("a workspace name is 1 to 80 characters")
	ErrAlreadyInvited  = errors.New("already invited")
)

// Config optionally restricts sign-in; empty lists admit every verified email.
type Config struct {
	AllowedEmailDomains []string
	AllowedEmails       []string
}

type Service struct {
	store storage.WorkspaceStore
	cfg   Config
}

func NewService(store storage.WorkspaceStore, cfg Config) *Service {
	return &Service{store: store, cfg: cfg}
}

// SignIn returns the user a person acts as. Accounts provisioned for their
// email become theirs and pending invites are accepted; people keep landing
// in their first workspace, a newcomer lands in a provisioned account or else
// the inviting workspace, and anyone else gets a workspace of their own.
func (s *Service) SignIn(ctx context.Context, id auth.Identity) (storage.User, error) {
	if !id.EmailVerified || id.Email == "" {
		return storage.User{}, ErrEmailUnverified
	}
	if !s.allowed(id.Email) {
		return storage.User{}, ErrNotAllowed
	}
	identity := storage.Identity{Issuer: id.Issuer, Subject: id.Subject}
	users, err := s.store.UsersByIdentity(ctx, id.Issuer, id.Subject)
	if err != nil {
		return storage.User{}, err
	}
	claimed, err := s.store.ShareIdentity(ctx, emailIdentity(id.Email), identity)
	if err != nil {
		return storage.User{}, err
	}
	joined, err := s.acceptInvites(ctx, identity, member(id))
	switch {
	case err != nil:
		return storage.User{}, err
	case len(users) > 0:
		return users[0], nil
	case len(claimed) > 0:
		return claimed[0], nil
	case joined != nil:
		return *joined, nil
	}
	return s.createOwned(ctx, member(id), identity)
}

// Provision returns the account a deployment declares for the email: its
// first user, or a new one with a workspace of its own that whoever signs in
// with that verified email takes over.
func (s *Service) Provision(ctx context.Context, email string) (storage.User, error) {
	users, err := s.store.UsersByEmail(ctx, email)
	if err != nil {
		return storage.User{}, err
	}
	if len(users) > 0 {
		return users[0], nil
	}
	return s.createOwned(ctx, member(auth.Identity{Email: email}), emailIdentity(email))
}

// emailIdentity stands for anyone who proves they hold the address, which is
// how a provisioned account awaits its person.
func emailIdentity(email string) storage.Identity {
	return storage.Identity{Issuer: "email", Subject: strings.ToLower(email)}
}

// createOwned starts a person as the admin of a workspace of their own.
func (s *Service) createOwned(ctx context.Context, owner storage.NewUser, identity storage.Identity) (storage.User, error) {
	owner.Admin = true
	return s.store.CreateOwnedWorkspace(ctx, "Personal", owner, identity, records.StandardObjects)
}

func (s *Service) allowed(email string) bool {
	if len(s.cfg.AllowedEmailDomains) == 0 && len(s.cfg.AllowedEmails) == 0 {
		return true
	}
	_, domain, _ := strings.Cut(email, "@")
	return slices.ContainsFunc(s.cfg.AllowedEmails, func(e string) bool { return strings.EqualFold(e, email) }) ||
		slices.ContainsFunc(s.cfg.AllowedEmailDomains, func(d string) bool { return strings.EqualFold(d, domain) })
}

// acceptInvites joins every workspace inviting the member's email that the
// identity is not already in, returning the last one joined.
func (s *Service) acceptInvites(ctx context.Context, identity storage.Identity, m storage.NewUser) (*storage.User, error) {
	invites, err := s.store.InvitesByEmail(ctx, m.Email)
	if err != nil || len(invites) == 0 {
		return nil, err
	}
	users, err := s.store.UsersByIdentity(ctx, identity.Issuer, identity.Subject)
	if err != nil {
		return nil, err
	}
	var joined *storage.User
	for _, invite := range invites {
		if slices.ContainsFunc(users, func(u storage.User) bool { return u.WorkspaceID == invite.WorkspaceID }) {
			if err := s.store.DeleteInvite(ctx, invite.WorkspaceID, invite.ID); err != nil {
				return nil, err
			}
			continue
		}
		user, err := s.store.JoinWorkspace(ctx, invite, m, identity)
		if err != nil {
			return nil, err
		}
		joined = &user
	}
	return joined, nil
}

// Members lists the actor's workspace members and pending invites.
func (s *Service) Members(ctx context.Context, actor auth.Actor) ([]storage.User, []storage.WorkspaceInvite, error) {
	users, err := s.store.Users(ctx, actor.WorkspaceID)
	if err != nil {
		return nil, nil, err
	}
	invites, err := s.store.Invites(ctx, actor.WorkspaceID)
	return users, invites, err
}

// Invite lets an admin invite an email into their workspace; the person
// joins when they next sign in.
func (s *Service) Invite(ctx context.Context, actor auth.Actor, email string) (storage.WorkspaceInvite, error) {
	if err := s.requireAdmin(ctx, actor); err != nil {
		return storage.WorkspaceInvite{}, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if local, domain, ok := strings.Cut(email, "@"); !ok || local == "" || !strings.Contains(domain, ".") {
		return storage.WorkspaceInvite{}, ErrInvalidEmail
	}
	invite, err := s.store.CreateInvite(ctx, actor.WorkspaceID, email, actor.UserID)
	if errors.Is(err, storage.ErrConflict) {
		return invite, fmt.Errorf("%s is %w", email, ErrAlreadyInvited)
	}
	return invite, err
}

// Rename lets an admin rename their workspace.
func (s *Service) Rename(ctx context.Context, actor auth.Actor, name string) (string, error) {
	if err := s.requireAdmin(ctx, actor); err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return "", ErrInvalidName
	}
	return name, s.store.RenameWorkspace(ctx, actor.WorkspaceID, name)
}

func (s *Service) requireAdmin(ctx context.Context, actor auth.Actor) error {
	user, err := s.store.UserByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !user.Admin {
		return ErrForbidden
	}
	return nil
}

func member(id auth.Identity) storage.NewUser {
	handle, _, _ := strings.Cut(id.Email, "@")
	name := strings.TrimSpace(id.Name)
	if name == "" {
		name = handle
	}
	user := storage.NewUser{Name: name, Email: id.Email}
	if id.Picture != "" {
		user.AvatarURL = &id.Picture
	}
	return user
}
