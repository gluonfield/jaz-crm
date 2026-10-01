// Package workspaces owns membership: who may sign in, the workspace a new
// person starts with, and invites.
package workspaces

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-tasks/auth"
)

var (
	ErrEmailUnverified = errors.New("your identity provider has not verified this email address")
	ErrNotAllowed      = errors.New("this email address is not allowed to sign in here")
	ErrForbidden       = errs.Invalid{Message: "only workspace admins can manage the workspace"}
	ErrInvalidEmail    = errs.Invalid{Message: "enter a valid email address"}
	ErrInvalidName     = errs.Invalid{Message: "a workspace name is 1 to 80 characters"}
	ErrNotMember       = errs.Invalid{Message: "you are not a member of that workspace"}
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
func (s *Service) SignIn(ctx context.Context, id signin.Identity) (storage.User, error) {
	if !id.EmailVerified || id.Email == "" {
		return storage.User{}, ErrEmailUnverified
	}
	if !s.allowed(id.Email) {
		return storage.User{}, ErrNotAllowed
	}
	identity := storage.Identity{Issuer: id.Issuer, Subject: id.Subject}
	for _, linked := range id.LinkedIdentities {
		if _, err := s.store.ShareIdentity(ctx, storage.Identity{Issuer: linked.Issuer, Subject: linked.Subject}, identity); err != nil {
			return storage.User{}, err
		}
	}
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
	return s.createOwned(ctx, member(signin.Identity{Email: email}), emailIdentity(email))
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
		return invite, errs.Invalidf("%s is already invited", email)
	}
	return invite, err
}

// Update lets an admin change the workspace's name and description; the
// description tells the triage agent which contacts belong in the CRM.
func (s *Service) Update(ctx context.Context, actor auth.Actor, name, description *string) (storage.Workspace, error) {
	if err := s.requireAdmin(ctx, actor); err != nil {
		return storage.Workspace{}, err
	}
	workspace, err := s.store.Workspace(ctx, actor.WorkspaceID)
	if err != nil {
		return workspace, err
	}
	if name != nil {
		workspace.Name = strings.TrimSpace(*name)
	}
	if description != nil {
		workspace.Description = strings.TrimSpace(*description)
	}
	if workspace.Name == "" || len([]rune(workspace.Name)) > 80 {
		return workspace, ErrInvalidName
	}
	if len([]rune(workspace.Description)) > 2000 {
		return workspace, errs.Invalidf("a workspace description is at most 2000 characters")
	}
	return workspace, s.store.UpdateWorkspace(ctx, workspace.ID, workspace.Name, workspace.Description)
}

// Memberships lists the workspaces the actor's person belongs to.
func (s *Service) Memberships(ctx context.Context, actor auth.Actor) ([]storage.Membership, error) {
	return s.store.Memberships(ctx, actor.UserID)
}

// Member returns the actor's person's membership of a workspace.
func (s *Service) Member(ctx context.Context, actor auth.Actor, workspaceID string) (storage.Membership, error) {
	memberships, err := s.store.Memberships(ctx, actor.UserID)
	if err != nil {
		return storage.Membership{}, err
	}
	i := slices.IndexFunc(memberships, func(m storage.Membership) bool { return m.WorkspaceID == workspaceID })
	if i < 0 {
		return storage.Membership{}, ErrNotMember
	}
	return memberships[i], nil
}

// In returns the actor acting in their workspace of that name, or as they
// are when no name is given.
func (s *Service) In(ctx context.Context, actor auth.Actor, name string) (auth.Actor, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return actor, nil
	}
	memberships, err := s.store.Memberships(ctx, actor.UserID)
	if err != nil {
		return actor, err
	}
	var named []storage.Membership
	names := make([]string, len(memberships))
	for i, m := range memberships {
		names[i] = m.Name
		if strings.EqualFold(m.Name, name) {
			named = append(named, m)
		}
	}
	switch len(named) {
	case 0:
		return actor, errs.Invalidf("you have no workspace named %q; yours are %s", name, strings.Join(names, ", "))
	case 1:
		return actor.In(named[0].UserID, named[0].WorkspaceID)
	}
	return actor, errs.Invalidf("several of your workspaces are named %q; rename one to tell them apart", name)
}

// Create starts a workspace with the actor's person as its admin, reachable
// by every identity they sign in with, and returns their user there.
func (s *Service) Create(ctx context.Context, actor auth.Actor, name string) (storage.User, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return storage.User{}, ErrInvalidName
	}
	me, err := s.store.UserByID(ctx, actor.UserID)
	if err != nil {
		return storage.User{}, err
	}
	identities, err := s.store.UserIdentities(ctx, actor.UserID)
	if err != nil {
		return storage.User{}, err
	}
	if len(identities) == 0 {
		return storage.User{}, errs.Invalidf("sign in to create a workspace")
	}
	owner := storage.NewUser{Name: me.Name, Email: me.Email, AvatarURL: me.AvatarURL, Admin: true}
	user, err := s.store.CreateOwnedWorkspace(ctx, name, owner, identities[0], records.StandardObjects)
	if err != nil {
		return user, err
	}
	for _, id := range identities[1:] {
		if _, err := s.store.ShareIdentity(ctx, identities[0], id); err != nil {
			return user, err
		}
	}
	return user, nil
}

// Workspace describes the actor's workspace.
func (s *Service) Workspace(ctx context.Context, actor auth.Actor) (storage.Workspace, error) {
	return s.store.Workspace(ctx, actor.WorkspaceID)
}

func (s *Service) Delete(ctx context.Context, actor auth.Actor, workspaceID, name string) error {
	if err := s.requireAdmin(ctx, actor); err != nil {
		return err
	}
	if workspaceID != actor.WorkspaceID {
		return errs.Invalidf("the current workspace changed; reopen Settings before deleting")
	}
	err := s.store.DeleteWorkspace(ctx, workspaceID, strings.TrimSpace(name))
	if errors.Is(err, storage.ErrNotFound) {
		return errs.Invalidf("enter the current workspace name to confirm deletion")
	}
	return err
}

func (s *Service) requireAdmin(ctx context.Context, actor auth.Actor) error {
	user, err := s.store.UserByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !user.Admin || user.WorkspaceID != actor.WorkspaceID {
		return ErrForbidden
	}
	return nil
}

func member(id signin.Identity) storage.NewUser {
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

func (s *Service) TriageSettings(ctx context.Context, actor auth.Actor) (storage.TriageSettings, error) {
	return s.store.TriageSettings(ctx, actor.WorkspaceID)
}

func (s *Service) UpdateTriageSettings(ctx context.Context, actor auth.Actor, settings storage.TriageSettings) error {
	if err := s.requireAdmin(ctx, actor); err != nil {
		return err
	}
	if settings.AutoKeepAi {
		workspace, err := s.store.Workspace(ctx, actor.WorkspaceID)
		if err != nil {
			return err
		}
		if workspace.Description == "" {
			return errs.Invalidf("enter Who belongs criteria before enabling AI decisions")
		}
	}
	return s.store.UpdateTriageSettings(ctx, actor.WorkspaceID, settings)
}
