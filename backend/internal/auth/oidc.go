package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	// RedirectURL is PUBLIC_URL/auth/callback.
	RedirectURL string
}

// OIDC signs people in with an OpenID Connect provider such as Google,
// Microsoft, Okta, Keycloak or Cognito. Discovery, and with it the JWKS, is
// fetched on first use so the server starts even when the provider is down.
type OIDC struct {
	cfg      OIDCConfig
	mu       sync.Mutex
	provider *oidc.Provider
}

func NewOIDC(cfg OIDCConfig) *OIDC {
	return &OIDC{cfg: cfg}
}

func (o *OIDC) Enabled() bool {
	return o.cfg.Issuer != "" && o.cfg.ClientID != ""
}

func (o *OIDC) setup(ctx context.Context) (*oidc.Provider, *oauth2.Config, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.provider == nil {
		provider, err := oidc.NewProvider(ctx, o.cfg.Issuer)
		if err != nil {
			return nil, nil, fmt.Errorf("discover %s: %w", o.cfg.Issuer, err)
		}
		o.provider = provider
	}
	return o.provider, &oauth2.Config{
		ClientID:     o.cfg.ClientID,
		ClientSecret: o.cfg.ClientSecret,
		Endpoint:     o.provider.Endpoint(),
		RedirectURL:  o.cfg.RedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}, nil
}

// AuthURL starts a sign-in bound to state, nonce and a PKCE verifier.
func (o *OIDC) AuthURL(ctx context.Context, state, nonce, verifier string) (string, error) {
	_, cfg, err := o.setup(ctx)
	if err != nil {
		return "", err
	}
	return cfg.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), nil
}

// Exchange redeems the provider's code and verifies the ID token.
func (o *OIDC) Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error) {
	provider, cfg, err := o.setup(ctx)
	if err != nil {
		return Identity{}, err
	}
	token, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("exchange code: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return Identity{}, errors.New("provider returned no id_token")
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: o.cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		return Identity{}, fmt.Errorf("verify id_token: %w", err)
	}
	if idToken.Nonce != nonce {
		return Identity{}, errors.New("id_token nonce mismatch")
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified any    `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, err
	}
	return Identity{
		Issuer:        idToken.Issuer,
		Subject:       idToken.Subject,
		Email:         strings.ToLower(claims.Email),
		EmailVerified: claims.EmailVerified == true || claims.EmailVerified == "true",
		Name:          claims.Name,
		Picture:       claims.Picture,
	}, nil
}
