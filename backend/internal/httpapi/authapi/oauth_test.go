package authapi_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/gluonfield/jaz-tasks/auth"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// The official MCP client discovers the authorization server from a 401,
// registers itself, runs PKCE through the consent screen and calls a tool.
func TestMCPClientAuthorizes(t *testing.T) {
	s := start(t, signin.OIDCConfig{}, workspaces.Config{})
	b := browser()
	s.ownerSession(t, b)
	const redirect = "http://127.0.0.1:9999/callback"
	handler, err := mcpauth.NewAuthorizationCodeHandler(&mcpauth.AuthorizationCodeHandlerConfig{
		DynamicClientRegistrationConfig: &mcpauth.DynamicClientRegistrationConfig{
			Metadata: &oauthex.ClientRegistrationMetadata{ClientName: "Test agent", RedirectURIs: []string{redirect}},
		},
		RedirectURL: redirect,
		AuthorizationCodeFetcher: func(_ context.Context, args *mcpauth.AuthorizationArgs) (*mcpauth.AuthorizationResult, error) {
			location := consent(t, b, args.URL)
			q := location.Query()
			return &mcpauth.AuthorizationResult{Code: q.Get("code"), State: q.Get("state"), Iss: q.Get("iss")}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "agent", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: s.url + "/mcp", OAuthHandler: handler}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_workspace"})
	if err != nil || res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, `"is_me":true`) {
		t.Fatalf("get_workspace: %+v %v", res, err)
	}
}

type tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Error        string `json:"error"`
}

func post(t *testing.T, endpoint string, form url.Values) (int, tokens) {
	t.Helper()
	res, err := http.PostForm(endpoint, form)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out tokens
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

// mcpStatus opens an MCP session with the credential, returning the status.
func mcpStatus(t *testing.T, base, authorization string) (int, http.Header) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, base+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode, res.Header
}

func TestTokenLifecycle(t *testing.T) {
	s := start(t, signin.OIDCConfig{}, workspaces.Config{})

	status, header := mcpStatus(t, s.url, "")
	if status != http.StatusUnauthorized || header.Get("WWW-Authenticate") != `Bearer resource_metadata="`+s.url+`/.well-known/oauth-protected-resource/mcp"` {
		t.Fatalf("anonymous mcp: %d %q", status, header.Get("WWW-Authenticate"))
	}
	var meta struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	getJSON(t, s.url+"/.well-known/oauth-protected-resource/mcp", &meta)
	var server struct {
		TokenEndpoint string   `json:"token_endpoint"`
		PKCE          []string `json:"code_challenge_methods_supported"`
	}
	getJSON(t, meta.AuthorizationServers[0]+"/.well-known/oauth-authorization-server", &server)
	if meta.Resource != s.url+"/mcp" || server.TokenEndpoint != s.url+"/oauth/token" || server.PKCE[0] != "S256" {
		t.Fatalf("metadata: %+v %+v", meta, server)
	}

	res, err := http.Post(s.url+"/oauth/register", "application/json", strings.NewReader(`{"client_name":"CLI","redirect_uris":["http://localhost:8765/cb"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var client struct {
		ClientID string `json:"client_id"`
	}
	_ = json.NewDecoder(res.Body).Decode(&client)
	res.Body.Close()
	if res.StatusCode != http.StatusCreated || client.ClientID == "" {
		t.Fatalf("register: %d", res.StatusCode)
	}

	b := browser()
	s.ownerSession(t, b)
	verifier := "a-very-long-and-random-code-verifier-for-this-test-0123456789"
	sum := sha256.Sum256([]byte(verifier))
	authorize := s.url + "/oauth/authorize?" + url.Values{
		"response_type": {"code"}, "client_id": {client.ClientID}, "redirect_uri": {"http://localhost:8765/cb"},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}, "state": {"xyz"},
	}.Encode()
	location := consent(t, b, authorize)
	if location.Query().Get("state") != "xyz" || location.Query().Get("iss") != s.url {
		t.Fatalf("redirect: %s", location)
	}
	code := location.Query().Get("code")
	exchange := url.Values{"grant_type": {"authorization_code"}, "client_id": {client.ClientID}, "code": {code}, "redirect_uri": {"http://localhost:8765/cb"}}

	wrong := url.Values{}
	for k, v := range exchange {
		wrong[k] = v
	}
	wrong.Set("code_verifier", "not-the-verifier")
	if status, out := post(t, s.url+"/oauth/token", wrong); status != http.StatusBadRequest || out.Error != "invalid_grant" {
		t.Fatalf("wrong verifier: %d %+v", status, out)
	}
	exchange.Set("code_verifier", verifier)
	if status, out := post(t, s.url+"/oauth/token", exchange); status != http.StatusBadRequest || out.Error != "invalid_grant" {
		t.Fatalf("a failed exchange must burn the code: %d %+v", status, out)
	}

	location = consent(t, b, authorize)
	exchange.Set("code", location.Query().Get("code"))
	status, first := post(t, s.url+"/oauth/token", exchange)
	if status != http.StatusOK || !strings.HasPrefix(first.AccessToken, "jc_at_") || !strings.HasPrefix(first.RefreshToken, "jc_rt_") {
		t.Fatalf("exchange: %d %+v", status, first)
	}
	if status, _ := post(t, s.url+"/oauth/token", exchange); status != http.StatusBadRequest {
		t.Fatalf("code reuse: %d", status)
	}
	if status, _ := mcpStatus(t, s.url, "Bearer "+first.AccessToken); status != http.StatusOK {
		t.Fatalf("access token on mcp: %d", status)
	}

	refresh := url.Values{"grant_type": {"refresh_token"}, "client_id": {client.ClientID}, "refresh_token": {first.RefreshToken}}
	status, second := post(t, s.url+"/oauth/token", refresh)
	if status != http.StatusOK || second.RefreshToken == first.RefreshToken {
		t.Fatalf("refresh: %d %+v", status, second)
	}
	if status, _ := mcpStatus(t, s.url, "Bearer "+first.AccessToken); status != http.StatusUnauthorized {
		t.Fatalf("rotated access token still works: %d", status)
	}
	if status, _ := mcpStatus(t, s.url, "Bearer "+second.AccessToken); status != http.StatusOK {
		t.Fatalf("new access token: %d", status)
	}
	if status, out := post(t, s.url+"/oauth/token", refresh); status != http.StatusBadRequest || out.Error != "invalid_grant" {
		t.Fatalf("refresh reuse: %d %+v", status, out)
	}
	if status, _ := mcpStatus(t, s.url, "Bearer "+second.AccessToken); status != http.StatusUnauthorized {
		t.Fatalf("reuse must revoke the whole grant: %d", status)
	}

	location = consent(t, b, authorize)
	exchange.Set("code", location.Query().Get("code"))
	_, third := post(t, s.url+"/oauth/token", exchange)
	if status, _ := post(t, s.url+"/oauth/revoke", url.Values{"token": {third.AccessToken}}); status != http.StatusOK {
		t.Fatalf("revoke: %d", status)
	}
	if status, _ := mcpStatus(t, s.url, "Bearer "+third.AccessToken); status != http.StatusUnauthorized {
		t.Fatalf("revoked token accepted: %d", status)
	}
	location = consent(t, b, authorize)
	exchange.Set("code", location.Query().Get("code"))
	_, fourth := post(t, s.url+"/oauth/token", exchange)
	post(t, s.url+"/oauth/revoke", url.Values{"token": {fourth.RefreshToken}})
	if status, _ := mcpStatus(t, s.url, "Bearer "+fourth.AccessToken); status != http.StatusUnauthorized {
		t.Fatalf("revoking a refresh token must end its access token: %d", status)
	}
	if status, _ := mcpStatus(t, s.url, "Bearer "+s.apiKey); status != http.StatusOK {
		t.Fatalf("API key: %d", status)
	}
}

func TestAuthorizeRejectsUnsafeRequests(t *testing.T) {
	s := start(t, signin.OIDCConfig{}, workspaces.Config{})
	b := browser()
	cases := map[string]struct {
		query  url.Values
		status int
	}{
		"unknown client": {url.Values{"client_id": {"nope"}, "redirect_uri": {"https://evil.test/cb"}}, http.StatusBadRequest},
	}
	for name, tc := range cases {
		res, err := b.Get(s.url + "/oauth/authorize?" + tc.query.Encode())
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != tc.status {
			t.Errorf("%s: %d", name, res.StatusCode)
		}
	}
	res, _ := http.Post(s.url+"/oauth/register", "application/json", strings.NewReader(`{"redirect_uris":["http://evil.test/cb"]}`))
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "invalid_redirect_uri") {
		t.Fatalf("plain http redirect registered: %d %s", res.StatusCode, body)
	}
	res, _ = http.Post(s.url+"/oauth/register", "application/json", strings.NewReader(`{"redirect_uris":["https://app.test/cb"]}`))
	var client struct {
		ClientID string `json:"client_id"`
	}
	_ = json.NewDecoder(res.Body).Decode(&client)
	res.Body.Close()
	res, _ = b.Get(s.url + "/oauth/authorize?" + url.Values{"response_type": {"code"}, "client_id": {client.ClientID}, "redirect_uri": {"https://app.test/cb"}, "state": {"s"}}.Encode())
	res.Body.Close()
	if location, _ := url.Parse(res.Header.Get("Location")); res.StatusCode != http.StatusFound || location.Query().Get("error") != "invalid_request" {
		t.Fatalf("missing PKCE: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	res, _ = b.Get(s.url + "/oauth/authorize?" + url.Values{"response_type": {"code"}, "client_id": {client.ClientID}, "redirect_uri": {"https://app.test/cb"}, "code_challenge": {"c"}, "code_challenge_method": {"S256"}}.Encode())
	res.Body.Close()
	if res.StatusCode != http.StatusFound || !strings.HasPrefix(res.Header.Get("Location"), "/auth/login?return_to=") {
		t.Fatalf("signed-out authorize: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
}

func getJSON(t *testing.T, endpoint string, v any) {
	t.Helper()
	res, err := http.Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatalf("%s: %v", endpoint, err)
	}
}

// Sign-in redirects stay on this site whatever return_to says.
func TestReturnToStaysOnSite(t *testing.T) {
	iss := newIssuer(t)
	s := start(t, signin.OIDCConfig{Issuer: iss.URL, ClientID: "client-1"}, workspaces.Config{})
	for raw, want := range map[string]string{
		"/people?x=1":       "/people?x=1",
		"/\t/evil.test":     "/",
		"//evil.test":       "/",
		"/\\evil.test":      "/",
		"https://evil.test": "/",
		"evil.test":         "/",
	} {
		raw = strings.ReplaceAll(raw, "\\t", "\t")
		res := iss.signInTo(t, s, browser(), "ada@example.com", raw)
		res.Body.Close()
		if got := res.Header.Get("Location"); got != want {
			t.Errorf("return_to %q redirected to %q", raw, got)
		}
	}
}

// Granting access is a person's decision: consent needs the browser session,
// never a bearer token or API key.
func TestConsentNeedsBrowserSession(t *testing.T) {
	s := start(t, signin.OIDCConfig{}, workspaces.Config{})
	res, _ := http.Post(s.url+"/oauth/register", "application/json", strings.NewReader(`{"redirect_uris":["http://127.0.0.1:9/cb"]}`))
	var client struct {
		ClientID string `json:"client_id"`
	}
	_ = json.NewDecoder(res.Body).Decode(&client)
	res.Body.Close()
	query := url.Values{"response_type": {"code"}, "client_id": {client.ClientID}, "redirect_uri": {"http://127.0.0.1:9/cb"}, "code_challenge": {"c"}, "code_challenge_method": {"S256"}}
	req, _ := http.NewRequest(http.MethodGet, s.url+"/oauth/authorize?"+query.Encode(), nil)
	req.Header.Set("Authorization", s.apiKey)
	res, err := browser().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusFound || !strings.HasPrefix(res.Header.Get("Location"), "/auth/login") {
		t.Fatalf("bearer-only consent: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	form := url.Values{"decision": {"allow"}, "csrf": {""}}
	for k, v := range query {
		form[k] = v
	}
	req, _ = http.NewRequest(http.MethodPost, s.url+"/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", s.apiKey)
	res, _ = browser().Do(req)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("bearer-only approval: %d", res.StatusCode)
	}
	query.Set("resource", "http://evil.test/mcp")
	b := browser()
	s.ownerSession(t, b)
	res, _ = b.Get(s.url + "/oauth/authorize?" + query.Encode())
	res.Body.Close()
	if location, _ := url.Parse(res.Header.Get("Location")); location.Query().Get("error") != "invalid_target" {
		t.Fatalf("foreign resource: %s", res.Header.Get("Location"))
	}
}

// Cookie-authenticated changes accept only JSON, which no cross-site form can send.
func TestSessionChangesNeedJSON(t *testing.T) {
	s := start(t, signin.OIDCConfig{}, workspaces.Config{})
	b := browser()
	s.ownerSession(t, b)
	for _, path := range []string{"/auth/api-keys", "/auth/logout", "/api/tools/list_objects"} {
		req, _ := http.NewRequest(http.MethodPost, s.url+path, strings.NewReader(`{"label":"x"}`))
		req.Header.Set("Content-Type", "text/plain")
		res, err := b.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnsupportedMediaType {
			t.Errorf("%s with text/plain: %d", path, res.StatusCode)
		}
	}
}
