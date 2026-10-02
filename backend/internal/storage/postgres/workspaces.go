package postgres

import (
	"context"
	"encoding/json"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	authdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/auth"
	recdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/records"
	"github.com/lithammer/shortuuid/v4"
)

func toInvite(r authdb.WorkspaceInvite) storage.WorkspaceInvite { return storage.WorkspaceInvite(r) }

func (s *Store) UsersByIdentity(ctx context.Context, issuer, subject string) ([]storage.User, error) {
	return many(toUser)(s.auth.UsersByIdentity(ctx, authdb.UsersByIdentityParams{Issuer: issuer, Subject: subject}))
}

func (s *Store) ShareIdentity(ctx context.Context, from, identity storage.Identity) ([]storage.User, error) {
	return many(toUser)(s.auth.ShareIdentity(ctx, authdb.ShareIdentityParams{
		Issuer: identity.Issuer, Subject: identity.Subject, FromIssuer: from.Issuer, FromSubject: from.Subject,
	}))
}

func (s *Store) CreateOwnedWorkspace(ctx context.Context, name string, owner storage.NewUser, identity storage.Identity, objects []storage.NewObject) (storage.User, error) {
	var user authdb.User
	err := s.tx(ctx, func(a *authdb.Queries, r *recdb.Queries) error {
		workspace, err := a.CreateWorkspace(ctx, name)
		if err != nil {
			return err
		}
		owner.WorkspaceID = workspace.ID
		if user, err = a.CreateAuthUser(ctx, authdb.CreateAuthUserParams(owner)); err != nil {
			return err
		}
		if err := a.LinkIdentity(ctx, authdb.LinkIdentityParams{Issuer: identity.Issuer, Subject: identity.Subject, UserID: user.ID}); err != nil {
			return err
		}
		return createObjects(ctx, r, workspace.ID, objects)
	})
	return one(toUser)(user, err)
}

func (s *Store) UpdateWorkspace(ctx context.Context, workspace storage.Workspace) error {
	return affected(s.auth.UpdateWorkspace(ctx, authdb.UpdateWorkspaceParams{ID: workspace.ID, Name: workspace.Name, Description: workspace.Description, CompanyPageID: workspace.CompanyPageID}))
}

func (s *Store) LockWorkspace(ctx context.Context, workspaceID string) error {
	_, err := s.auth.LockWorkspace(ctx, workspaceID)
	return mapError(err)
}

func (s *Store) DeleteWorkspace(ctx context.Context, id, name string) error {
	return affected(s.auth.DeleteWorkspace(ctx, authdb.DeleteWorkspaceParams{ID: id, Name: name}))
}

// createObjects creates every object before any attribute, so references
// resolve whatever order the objects come in.
func createObjects(ctx context.Context, r *recdb.Queries, workspaceID string, objects []storage.NewObject) error {
	ids := map[string]string{}
	for _, object := range objects {
		created, err := r.CreateObject(ctx, recdb.CreateObjectParams{WorkspaceID: workspaceID, Slug: object.Slug, Name: object.Name})
		if err != nil {
			return err
		}
		ids[object.Slug] = created.ID
	}
	for _, object := range objects {
		for _, attr := range object.Attributes {
			var target *string
			if attr.Target != "" {
				id := ids[attr.Target]
				target = &id
			}
			if _, err := r.CreateAttribute(ctx, recdb.CreateAttributeParams{
				ObjectID: ids[object.Slug], Slug: attr.Slug, Name: attr.Name, Type: attr.Type,
				Multi: attr.Multi, IsUnique: attr.IsUnique, TargetObjectID: target, Options: append([]string{}, attr.Options...),
			}); err != nil {
				return err
			}
		}
		for _, filter := range object.Filters {
			data, err := json.Marshal(filter.Filters)
			if err != nil {
				return err
			}
			if _, err := r.CreateSavedFilter(ctx, recdb.CreateSavedFilterParams{ID: shortuuid.New(), WorkspaceID: workspaceID, ObjectID: ids[object.Slug], Name: filter.Name, Query: filter.Query, Filters: data}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) JoinWorkspace(ctx context.Context, invite storage.WorkspaceInvite, member storage.NewUser, identity storage.Identity) (storage.User, error) {
	var user authdb.User
	err := s.tx(ctx, func(a *authdb.Queries, _ *recdb.Queries) error {
		if err := affected(a.DeleteInvite(ctx, authdb.DeleteInviteParams{WorkspaceID: invite.WorkspaceID, ID: invite.ID})); err != nil {
			return err
		}
		member.WorkspaceID = invite.WorkspaceID
		var err error
		if user, err = a.CreateAuthUser(ctx, authdb.CreateAuthUserParams(member)); err != nil {
			return err
		}
		return a.LinkIdentity(ctx, authdb.LinkIdentityParams{Issuer: identity.Issuer, Subject: identity.Subject, UserID: user.ID})
	})
	return one(toUser)(user, err)
}

func (s *Store) CreateInvite(ctx context.Context, workspaceID, email, invitedBy string) (storage.WorkspaceInvite, error) {
	return one(toInvite)(s.auth.CreateInvite(ctx, authdb.CreateInviteParams{WorkspaceID: workspaceID, Email: email, InvitedBy: &invitedBy}))
}

func (s *Store) Invites(ctx context.Context, workspaceID string) ([]storage.WorkspaceInvite, error) {
	return many(toInvite)(s.auth.ListInvites(ctx, workspaceID))
}

func (s *Store) InvitesByEmail(ctx context.Context, email string) ([]storage.WorkspaceInvite, error) {
	return many(toInvite)(s.auth.InvitesByEmail(ctx, email))
}

func (s *Store) DeleteInvite(ctx context.Context, workspaceID, id string) error {
	return affected(s.auth.DeleteInvite(ctx, authdb.DeleteInviteParams{WorkspaceID: workspaceID, ID: id}))
}

func (s *Store) TriageSettings(ctx context.Context, workspaceID string) (storage.TriageSettings, error) {
	settings, err := s.auth.GetTriageSettings(ctx, workspaceID)
	return storage.TriageSettings(settings), mapError(err)
}

func (s *Store) UpdateTriageSettings(ctx context.Context, workspaceID string, settings storage.TriageSettings) error {
	return affected(s.auth.UpdateTriageSettings(ctx, authdb.UpdateTriageSettingsParams{
		ID: workspaceID, AutoKeepEmail: settings.AutoKeepEmail, AutoKeepMeetings: settings.AutoKeepMeetings,
		AutoKeepRecords: settings.AutoKeepRecords, AutoKeepAi: settings.AutoKeepAi,
	}))
}
