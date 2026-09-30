package storage

import (
	"context"
	"time"
)

type Workspace struct {
	ID          string
	Name        string
	CreatedAt   time.Time
	Description string
}

// User is a person's membership of one workspace.
type User struct {
	ID          string
	WorkspaceID string
	Name        string
	Email       string
	AvatarURL   *string
	Admin       bool
	CreatedAt   time.Time
}

type NewUser struct {
	WorkspaceID string
	Name        string
	Email       string
	AvatarURL   *string
	Admin       bool
}

type APIKey struct {
	ID        string
	UserID    string
	Label     string
	Hint      string
	KeyHash   []byte
	CreatedAt time.Time
}

type OAuthClient struct {
	ID           string
	Name         string
	RedirectURIs []string
	CreatedAt    time.Time
}

type OAuthCode struct {
	CodeHash      []byte
	ClientID      string
	UserID        string
	RedirectURI   string
	CodeChallenge string
	Scope         string
	ExpiresAt     time.Time
}

type OAuthGrant struct {
	ID        string
	ClientID  string
	UserID    string
	Scope     string
	CreatedAt time.Time
	RevokedAt *time.Time
}

type OAuthToken struct {
	TokenHash []byte
	GrantID   string
	Kind      string
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// OAuthGrantSummary is an authorized application as its user sees it.
type OAuthGrantSummary struct {
	ID         string
	ClientName string
	CreatedAt  time.Time
	LastUsedAt time.Time
}

type NewOAuthToken struct {
	TokenHash []byte
	GrantID   string
	Kind      string
	ExpiresAt time.Time
}

type AuthStore interface {
	UserByAPIKey(ctx context.Context, keyHash []byte) (User, error)
	CreateAPIKey(ctx context.Context, userID, label, hint string, keyHash []byte) (APIKey, error)
	// ReplaceAPIKey makes the key the user's only one with the label.
	ReplaceAPIKey(ctx context.Context, userID, label, hint string, keyHash []byte) (APIKey, error)
	APIKeys(ctx context.Context, userID string) ([]APIKey, error)
	DeleteAPIKey(ctx context.Context, userID, id string) error
	UsersByEmail(ctx context.Context, email string) ([]User, error)
	UserByID(ctx context.Context, id string) (User, error)
	Workspace(ctx context.Context, id string) (Workspace, error)

	CreateSession(ctx context.Context, tokenHash []byte, userID string, expiresAt time.Time) error
	UserBySession(ctx context.Context, tokenHash []byte) (User, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error

	CreateOAuthClient(ctx context.Context, client OAuthClient) (OAuthClient, error)
	OAuthClient(ctx context.Context, id string) (OAuthClient, error)
	CreateOAuthCode(ctx context.Context, code OAuthCode) error
	// ConsumeOAuthCode deletes and returns a code, so it works exactly once.
	ConsumeOAuthCode(ctx context.Context, codeHash []byte) (OAuthCode, error)
	// CreateOAuthGrant starts a grant with its first tokens.
	CreateOAuthGrant(ctx context.Context, clientID, userID, scope string, tokens []NewOAuthToken) (OAuthGrant, error)
	OAuthToken(ctx context.Context, tokenHash []byte) (OAuthToken, OAuthGrant, error)
	// RotateOAuthTokens revokes the used token and every live token of its
	// grant, then issues replacements; a used token already revoked returns
	// ErrReused after revoking the grant.
	RotateOAuthTokens(ctx context.Context, grantID string, usedHash []byte, tokens []NewOAuthToken) error
	// RevokeOAuthToken revokes a token; a refresh token takes every token of
	// its grant with it (RFC 7009).
	RevokeOAuthToken(ctx context.Context, tokenHash []byte) error
	UserByAccessToken(ctx context.Context, tokenHash []byte) (User, error)
	OAuthGrants(ctx context.Context, userID string) ([]OAuthGrantSummary, error)
	RevokeOAuthGrant(ctx context.Context, userID, grantID string) error
}

// Identity links a person's OIDC subject to one user row per workspace.
type Identity struct {
	Issuer    string
	Subject   string
	UserID    string
	CreatedAt time.Time
}

type WorkspaceInvite struct {
	ID          string
	WorkspaceID string
	Email       string
	InvitedBy   *string
	CreatedAt   time.Time
}

type WorkspaceStore interface {
	Workspace(ctx context.Context, id string) (Workspace, error)
	UserByID(ctx context.Context, id string) (User, error)
	Users(ctx context.Context, workspaceID string) ([]User, error)
	UsersByIdentity(ctx context.Context, issuer, subject string) ([]User, error)
	UsersByEmail(ctx context.Context, email string) ([]User, error)
	// ShareIdentity links identity to every user from signs in as.
	ShareIdentity(ctx context.Context, from, identity Identity) ([]User, error)
	// CreateOwnedWorkspace creates a workspace, its owner linked to the
	// identity, and its objects, atomically.
	CreateOwnedWorkspace(ctx context.Context, name string, owner NewUser, identity Identity, objects []NewObject) (User, error)
	UpdateWorkspace(ctx context.Context, id, name, description string) error
	// JoinWorkspace turns an invite into a member linked to the identity.
	JoinWorkspace(ctx context.Context, invite WorkspaceInvite, member NewUser, identity Identity) (User, error)
	CreateInvite(ctx context.Context, workspaceID, email, invitedBy string) (WorkspaceInvite, error)
	Invites(ctx context.Context, workspaceID string) ([]WorkspaceInvite, error)
	InvitesByEmail(ctx context.Context, email string) ([]WorkspaceInvite, error)
	DeleteInvite(ctx context.Context, workspaceID, id string) error
}
