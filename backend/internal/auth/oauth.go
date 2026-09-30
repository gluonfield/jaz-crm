package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

const (
	codeTTL    = 10 * time.Minute
	accessTTL  = time.Hour
	refreshTTL = 30 * 24 * time.Hour
)

// OAuthError is an RFC 6749 error response.
type OAuthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

func (e OAuthError) Error() string {
	return e.Code + ": " + e.Description
}

func oauthErr(code, description string) error {
	return OAuthError{Code: code, Description: description}
}

func (s *Service) Issuer() string {
	return s.cfg.PublicURL
}

// ResourceMetadataURL is the RFC 9728 document for a resource path such as /mcp.
func (s *Service) ResourceMetadataURL(path string) string {
	return s.cfg.PublicURL + "/.well-known/oauth-protected-resource" + path
}

// RegisterClient registers a public client (RFC 7591); clients prove
// possession with PKCE instead of a secret.
func (s *Service) RegisterClient(ctx context.Context, name string, redirectURIs []string) (storage.OAuthClient, error) {
	if len(redirectURIs) == 0 {
		return storage.OAuthClient{}, oauthErr("invalid_redirect_uri", "at least one redirect_uri is required")
	}
	for _, uri := range redirectURIs {
		if !validRedirect(uri) {
			return storage.OAuthClient{}, oauthErr("invalid_redirect_uri", "redirect URIs must be https, http on a loopback host, or a private-use scheme: "+uri)
		}
	}
	if name = strings.TrimSpace(name); name == "" {
		name = "Unnamed application"
	}
	return s.store.CreateOAuthClient(ctx, storage.OAuthClient{ID: secret("jc_client_"), Name: name, RedirectURIs: redirectURIs})
}

func validRedirect(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Fragment != "" || u.Scheme == "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return u.Host != ""
	case "http":
		ip := net.ParseIP(u.Hostname())
		return u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()
	case "javascript", "data", "file", "vbscript":
		return false
	}
	return true
}

// AuthorizeRequest is the query of an authorization request.
type AuthorizeRequest struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	State               string
	Scope               string
	Resource            string
}

// ErrBadRedirect means the client or redirect URI cannot be trusted, so the
// error must be shown to the user rather than redirected.
var ErrBadRedirect = errors.New("unknown client or unregistered redirect_uri")

// CheckAuthorize validates a request. Errors other than ErrBadRedirect are
// OAuthErrors to send back to the client's redirect URI.
func (s *Service) CheckAuthorize(ctx context.Context, req AuthorizeRequest) (storage.OAuthClient, error) {
	client, err := s.store.OAuthClient(ctx, req.ClientID)
	if errors.Is(err, storage.ErrNotFound) || err == nil && !slices.Contains(client.RedirectURIs, req.RedirectURI) {
		return client, ErrBadRedirect
	}
	switch {
	case err != nil:
		return client, err
	case req.ResponseType != "code":
		return client, oauthErr("unsupported_response_type", "only the code response type is supported")
	case req.CodeChallenge == "" || req.CodeChallengeMethod != "S256":
		return client, oauthErr("invalid_request", "PKCE with code_challenge_method=S256 is required")
	case req.Resource != "" && !slices.Contains([]string{s.cfg.PublicURL, s.cfg.PublicURL + "/mcp", s.cfg.PublicURL + "/graphql"}, req.Resource):
		return client, oauthErr("invalid_target", "resource must be "+s.cfg.PublicURL+"/mcp or "+s.cfg.PublicURL+"/graphql")
	}
	return client, nil
}

// Approve issues an authorization code and returns where to send the browser.
func (s *Service) Approve(ctx context.Context, actor Actor, req AuthorizeRequest) (string, error) {
	if _, err := s.CheckAuthorize(ctx, req); err != nil {
		return "", err
	}
	code := secret("jc_code_")
	if err := s.store.CreateOAuthCode(ctx, storage.OAuthCode{
		CodeHash:      hash(code),
		ClientID:      req.ClientID,
		UserID:        actor.UserID,
		RedirectURI:   req.RedirectURI,
		CodeChallenge: req.CodeChallenge,
		Scope:         req.Scope,
		ExpiresAt:     s.now().Add(codeTTL),
	}); err != nil {
		return "", err
	}
	return s.Redirect(req, url.Values{"code": {code}}), nil
}

// Redirect sends a response to the client, echoing state and naming the
// issuer (RFC 9207).
func (s *Service) Redirect(req AuthorizeRequest, params url.Values) string {
	if req.State != "" {
		params.Set("state", req.State)
	}
	params.Set("iss", s.cfg.PublicURL)
	sep := "?"
	if strings.Contains(req.RedirectURI, "?") {
		sep = "&"
	}
	return req.RedirectURI + sep + params.Encode()
}

// TokenRequest is the form posted to the token endpoint.
type TokenRequest struct {
	GrantType    string
	ClientID     string
	Code         string
	RedirectURI  string
	CodeVerifier string
	RefreshToken string
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope,omitempty"`
}

func (s *Service) Token(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	switch req.GrantType {
	case "authorization_code":
		return s.exchangeCode(ctx, req)
	case "refresh_token":
		return s.refresh(ctx, req)
	}
	return TokenResponse{}, oauthErr("unsupported_grant_type", "use authorization_code or refresh_token")
}

func (s *Service) exchangeCode(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	code, err := s.store.ConsumeOAuthCode(ctx, hash(req.Code))
	if errors.Is(err, storage.ErrNotFound) {
		return TokenResponse{}, oauthErr("invalid_grant", "unknown or already used authorization code")
	}
	if err != nil {
		return TokenResponse{}, err
	}
	switch {
	case s.now().After(code.ExpiresAt):
		return TokenResponse{}, oauthErr("invalid_grant", "authorization code expired")
	case code.ClientID != req.ClientID || code.RedirectURI != req.RedirectURI:
		return TokenResponse{}, oauthErr("invalid_grant", "client_id or redirect_uri does not match the authorization request")
	case !verifierMatches(req.CodeVerifier, code.CodeChallenge):
		return TokenResponse{}, oauthErr("invalid_grant", "code_verifier does not match code_challenge")
	}
	res, tokens := s.issue(code.Scope)
	_, err = s.store.CreateOAuthGrant(ctx, code.ClientID, code.UserID, code.Scope, tokens)
	return res, err
}

func verifierMatches(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	return verifier != "" && subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(challenge)) == 1
}

// refresh rotates the grant's tokens. Presenting a refresh token that was
// already rotated revokes the grant, as OAuth 2.1 requires for public clients.
func (s *Service) refresh(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	invalid := oauthErr("invalid_grant", "refresh token is invalid, expired or revoked")
	token, grant, err := s.store.OAuthToken(ctx, hash(req.RefreshToken))
	if errors.Is(err, storage.ErrNotFound) || err == nil && token.Kind != "refresh" {
		return TokenResponse{}, invalid
	}
	if err != nil {
		return TokenResponse{}, err
	}
	if grant.ClientID != req.ClientID {
		return TokenResponse{}, oauthErr("invalid_grant", "refresh token was issued to another client")
	}
	if grant.RevokedAt != nil || s.now().After(token.ExpiresAt) {
		return TokenResponse{}, invalid
	}
	res, tokens := s.issue(grant.Scope)
	if err := s.store.RotateOAuthTokens(ctx, grant.ID, token.TokenHash, tokens); errors.Is(err, storage.ErrReused) {
		return TokenResponse{}, invalid
	} else if err != nil {
		return TokenResponse{}, err
	}
	return res, nil
}

func (s *Service) issue(scope string) (TokenResponse, []storage.NewOAuthToken) {
	access, refresh := secret(accessTokenPrefix), secret(refreshTokenPrefix)
	now := s.now()
	return TokenResponse{
			AccessToken:  access,
			TokenType:    "Bearer",
			ExpiresIn:    int(accessTTL.Seconds()),
			RefreshToken: refresh,
			Scope:        scope,
		}, []storage.NewOAuthToken{
			{TokenHash: hash(access), Kind: "access", ExpiresAt: now.Add(accessTTL)},
			{TokenHash: hash(refresh), Kind: "refresh", ExpiresAt: now.Add(refreshTTL)},
		}
}

// Revoke implements RFC 7009: unknown tokens are not an error.
func (s *Service) Revoke(ctx context.Context, token string) error {
	return s.store.RevokeOAuthToken(ctx, hash(token))
}

func (s *Service) Grants(ctx context.Context, actor Actor) ([]storage.OAuthGrantSummary, error) {
	return s.store.OAuthGrants(ctx, actor.UserID)
}

func (s *Service) RevokeGrant(ctx context.Context, actor Actor, id string) error {
	return s.store.RevokeOAuthGrant(ctx, actor.UserID, id)
}
