package google

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

// Scopes a CRM connection requests in one grant.
var Scopes = []string{
	"https://www.googleapis.com/auth/gmail.readonly",
	"https://www.googleapis.com/auth/calendar.readonly",
	"https://www.googleapis.com/auth/meetings.space.readonly",
	"https://www.googleapis.com/auth/contacts.other.readonly",
}

type OAuthConfig struct {
	ClientID, ClientSecret, RedirectURL string
	// TokenURL overrides Google's token endpoint, for tests.
	TokenURL string
}

func (c OAuthConfig) AuthURL(state, verifier string) string {
	return c.config().AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
		oauth2.S256ChallengeOption(verifier),
	)
}

func (c OAuthConfig) Exchange(ctx context.Context, code, verifier string) (*oauth2.Token, error) {
	return c.config().Exchange(ctx, code, oauth2.VerifierOption(verifier))
}

// Client refreshes access tokens with ctx, so ctx must outlive the client.
func (c OAuthConfig) Client(ctx context.Context, refreshToken string) *http.Client {
	return c.config().Client(ctx, &oauth2.Token{RefreshToken: refreshToken})
}

func (c OAuthConfig) config() *oauth2.Config {
	token := c.TokenURL
	if token == "" {
		token = "https://oauth2.googleapis.com/token"
	}
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURL,
		Scopes:       Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:  token,
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}
