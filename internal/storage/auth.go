package storage

import (
	"context"
	"time"
)

type Workspace struct {
	ID                string
	Name              string
	CreatedAt         time.Time
	Description       string
	AutoKeepEmail     bool
	AutoKeepMeetings  bool
	AutoKeepRecords   bool
	AutoKeepAi        bool
	DraftingWebAccess bool
	CompanyPageIDs    []string
	Timezone          string
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
	// Addresses are other addresses the member sends from.
	Addresses []string
}

type NewUser struct {
	WorkspaceID string
	Name        string
	Email       string
	AvatarURL   *string
	Admin       bool
	ID          string
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
	// UpdateSessionUser moves a session to another of its person's users.
	UpdateSessionUser(ctx context.Context, tokenHash []byte, userID string) error
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
	// UserByAccessToken returns the token's user and grant.
	UserByAccessToken(ctx context.Context, tokenHash []byte) (User, string, error)
	// UpdateOAuthGrantUser moves a grant to another of its person's users.
	UpdateOAuthGrantUser(ctx context.Context, grantID, userID string) error
	OAuthGrants(ctx context.Context, userID string) ([]OAuthGrantSummary, error)
	RevokeOAuthGrant(ctx context.Context, userID, grantID string) error
}

// Membership is one of a person's workspaces with their user there.
type Membership struct {
	UserID      string
	WorkspaceID string
	Name        string
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

type TriageSettings struct {
	AutoKeepEmail    bool `json:"auto_keep_email"`
	AutoKeepMeetings bool `json:"auto_keep_meetings"`
	AutoKeepRecords  bool `json:"auto_keep_records"`
	AutoKeepAi       bool `json:"auto_keep_ai"`
}

type WorkspaceUpdate struct {
	Timezone          *string
	Name              *string
	Description       *string
	CompanyPageIDs    []string
	DraftingWebAccess *bool
}

type WorkspaceStore interface {
	Workspace(ctx context.Context, id string) (Workspace, error)
	UserByID(ctx context.Context, id string) (User, error)
	Users(ctx context.Context, workspaceID string) ([]User, error)
	UsersByIdentity(ctx context.Context, issuer, subject string) ([]User, error)
	UsersByEmail(ctx context.Context, email string) ([]User, error)
	UserIdentities(ctx context.Context, userID string) ([]Identity, error)
	// Memberships lists the workspaces of a user's person.
	Memberships(ctx context.Context, userID string) ([]Membership, error)
	// ShareIdentity links identity to every user from signs in as.
	ShareIdentity(ctx context.Context, from, identity Identity) ([]User, error)
	// CreateOwnedWorkspace creates a workspace, its owner linked to the
	// identity, and its objects, atomically.
	CreateOwnedWorkspace(ctx context.Context, name string, owner NewUser, identity Identity, objects []NewObject) (User, error)
	UpdateWorkspace(ctx context.Context, workspaceID string, update WorkspaceUpdate) error
	SetUserAddresses(ctx context.Context, workspaceID, userID string, addresses []string) error
	TriageSettings(ctx context.Context, workspaceID string) (TriageSettings, error)
	UpdateTriageSettings(ctx context.Context, workspaceID string, settings TriageSettings) error
	DeleteWorkspace(ctx context.Context, id, name string) error
	// JoinWorkspace turns an invite into a member linked to the identity.
	JoinWorkspace(ctx context.Context, invite WorkspaceInvite, member NewUser, identity Identity) (User, error)
	CreateInvite(ctx context.Context, workspaceID, email, invitedBy string) (WorkspaceInvite, error)
	Invites(ctx context.Context, workspaceID string) ([]WorkspaceInvite, error)
	InvitesByEmail(ctx context.Context, email string) ([]WorkspaceInvite, error)
	DeleteInvite(ctx context.Context, workspaceID, id string) error
}
