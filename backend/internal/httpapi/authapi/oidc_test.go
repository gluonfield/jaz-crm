package authapi_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// issuer is a fake OpenID provider: discovery, JWKS and a token endpoint
// that signs whatever identity the test queued for the next code.
type issuer struct {
	*httptest.Server
	key    *rsa.PrivateKey
	claims map[string]map[string]any
}

func newIssuer(t *testing.T) *issuer {
	t.Helper()
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	iss := &issuer{key: key, claims: map[string]map[string]any{}}
	mux := http.NewServeMux()
	iss.Server = httptest.NewServer(mux)
	t.Cleanup(iss.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                iss.URL,
			"authorization_endpoint":                iss.URL + "/authorize",
			"token_endpoint":                        iss.URL + "/token",
			"jwks_uri":                              iss.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		claims := iss.claims[r.FormValue("code")]
		if claims == nil || r.FormValue("code_verifier") == "" {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		signer, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"))
		token, _ := jwt.Signed(signer).Claims(claims).Serialize()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "id_token": token, "expires_in": 3600})
	})
	return iss
}

// signIn walks a browser through /auth/login, the provider and the callback
// as the given person, returning the callback response.
func (iss *issuer) signIn(t *testing.T, s stack, b *http.Client, email string, verified bool) *http.Response {
	t.Helper()
	return iss.sign(t, s, b, email, verified, "/")
}

// signInTo signs a verified person in, asking to return to returnTo.
func (iss *issuer) signInTo(t *testing.T, s stack, b *http.Client, email, returnTo string) *http.Response {
	t.Helper()
	return iss.sign(t, s, b, email, true, returnTo)
}

func (iss *issuer) sign(t *testing.T, s stack, b *http.Client, email string, verified bool, returnTo string) *http.Response {
	t.Helper()
	res, err := b.Get(s.url + "/auth/login?" + url.Values{"return_to": {returnTo}}.Encode())
	if err != nil || res.StatusCode != http.StatusFound {
		t.Fatalf("login: %v %v", res.StatusCode, err)
	}
	res.Body.Close()
	authorize, _ := url.Parse(res.Header.Get("Location"))
	q := authorize.Query()
	if !strings.HasPrefix(authorize.String(), iss.URL+"/authorize") || q.Get("redirect_uri") != s.url+"/auth/callback" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("provider redirect: %s", authorize)
	}
	code := "code-" + email
	iss.claims[code] = map[string]any{
		"iss": iss.URL, "sub": "sub-" + email, "aud": "client-1", "nonce": q.Get("nonce"),
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		"email": email, "email_verified": verified, "name": "Casey " + strings.Split(email, "@")[0], "picture": "https://img.test/a.png",
	}
	res, err = b.Get(s.url + "/auth/callback?" + url.Values{"code": {code}, "state": {q.Get("state")}}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	return res
}

type me struct {
	Email     string
	Admin     bool
	Workspace struct {
		ID   string
		Name string
	}
}

// viewer reads who the browser acts as and in which workspace.
func viewer(t *testing.T, s stack, b *http.Client) me {
	t.Helper()
	res, err := b.Get(s.url + "/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out me
	_ = json.NewDecoder(res.Body).Decode(&out)
	return out
}

func session(t *testing.T, s stack, b *http.Client, method, path, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, s.url+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := b.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(out)
}

// Anyone with a verified email signs up into a workspace of their own, and
// a returning person lands where they did before.
func TestOIDCSignUpGivesEachPersonAWorkspace(t *testing.T) {
	iss := newIssuer(t)
	s := start(t, auth.OIDCConfig{Issuer: iss.URL, ClientID: "client-1", ClientSecret: "secret"}, workspaces.Config{})

	ada := browser()
	res := iss.signIn(t, s, ada, "ada@example.com", true)
	res.Body.Close()
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/" {
		t.Fatalf("sign-in: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	first := viewer(t, s, ada)
	if first.Email != "ada@example.com" || !first.Admin || first.Workspace.ID == "" {
		t.Fatalf("new workspace: %+v", first)
	}
	if text := callText(t, s.agent(t, ada), "list_objects", nil); !strings.Contains(text, `"slug":"people"`) || !strings.Contains(text, `"slug":"companies"`) {
		t.Fatalf("a new workspace starts with people and companies: %s", text)
	}

	grace := browser()
	res = iss.signIn(t, s, grace, "grace@example.com", true)
	res.Body.Close()
	second := viewer(t, s, grace)
	if !second.Admin || second.Workspace.ID == first.Workspace.ID {
		t.Fatalf("second person must get another workspace: %+v vs %+v", second, first)
	}

	again := browser()
	res = iss.signIn(t, s, again, "ada@example.com", true)
	res.Body.Close()
	if got := viewer(t, s, again); got.Workspace.ID != first.Workspace.ID {
		t.Fatalf("returning person lands elsewhere: %+v", got)
	}

	res = iss.signIn(t, s, browser(), "eve@example.com", false)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("unverified email: %d", res.StatusCode)
	}

	if status, _ := session(t, s, ada, http.MethodPost, "/auth/logout", ""); status != http.StatusNoContent {
		t.Fatalf("logout: %d", status)
	}
	if got := viewer(t, s, ada); got.Email != "" {
		t.Fatalf("session survived logout: %+v", got)
	}
}

func TestOIDCSignInAllowlist(t *testing.T) {
	iss := newIssuer(t)
	s := start(t, auth.OIDCConfig{Issuer: iss.URL, ClientID: "client-1"}, workspaces.Config{AllowedEmailDomains: []string{"ml.ink"}, AllowedEmails: []string{"guest@example.com"}})
	for email, allowed := range map[string]bool{"ana@ml.ink": true, "guest@example.com": true, "other@example.com": false} {
		b := browser()
		res := iss.signIn(t, s, b, email, true)
		res.Body.Close()
		if got := viewer(t, s, b); allowed != (got.Email == email) {
			t.Errorf("%s: status %d, viewer %+v", email, res.StatusCode, got)
		}
	}
}

// Invites are the only way into someone else's workspace: new people land in
// the inviting workspace, and an invite never moves where a person lands.
func TestInvites(t *testing.T) {
	iss := newIssuer(t)
	s := start(t, auth.OIDCConfig{Issuer: iss.URL, ClientID: "client-1"}, workspaces.Config{})
	owner := browser()
	iss.signIn(t, s, owner, "owner@example.com", true).Body.Close()
	home := viewer(t, s, owner)
	admin := s.agent(t, owner)

	if text := callText(t, admin, "invite_member", map[string]any{"email": "Bob@Example.com"}); !strings.Contains(text, `"invited":"bob@example.com"`) {
		t.Fatalf("invite: %s", text)
	}
	bob := browser()
	iss.signIn(t, s, bob, "bob@example.com", true).Body.Close()
	if got := viewer(t, s, bob); got.Workspace.ID != home.Workspace.ID || got.Admin {
		t.Fatalf("invited person should join as a member: %+v", got)
	}
	if text := callText(t, s.agent(t, bob), "invite_member", map[string]any{"email": "x@example.com"}); !strings.Contains(text, workspaces.ErrForbidden.Error()) {
		t.Fatalf("members cannot invite: %s", text)
	}
	if text := callText(t, s.agent(t, bob), "rename_workspace", map[string]any{"name": "Mine"}); !strings.Contains(text, workspaces.ErrForbidden.Error()) {
		t.Fatalf("members cannot rename: %s", text)
	}
	callText(t, admin, "rename_workspace", map[string]any{"name": " CAS "})
	if got := viewer(t, s, bob); got.Workspace.Name != "CAS" {
		t.Fatalf("renamed workspace: %+v", got)
	}
	if text := callText(t, admin, "list_members", nil); !strings.Contains(text, `"email":"bob@example.com"`) || !strings.Contains(text, `"invited":[]`) {
		t.Fatalf("members after joining: %s", text)
	}

	carol := browser()
	iss.signIn(t, s, carol, "carol@example.com", true).Body.Close()
	own := viewer(t, s, carol)
	callText(t, admin, "invite_member", map[string]any{"email": "carol@example.com"})
	again := browser()
	iss.signIn(t, s, again, "carol@example.com", true).Body.Close()
	if got := viewer(t, s, again); got.Workspace.ID != own.Workspace.ID {
		t.Fatalf("an invite must not move where a person lands: %+v", got)
	}
}
