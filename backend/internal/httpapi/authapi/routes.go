package authapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/gluonfield/jaz-tasks/auth"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func (h *Handler) routes() {
	h.mux.HandleFunc("GET /{$}", h.home)
	h.mux.HandleFunc("GET /auth/me", h.me)
	h.mux.HandleFunc("POST /auth/logout", h.logout)
	h.mux.HandleFunc("GET /auth/api-keys", h.listKeys)
	h.mux.HandleFunc("POST /auth/api-keys", h.createKey)
	h.mux.HandleFunc("DELETE /auth/api-keys/{id}", h.deleteKey)
	h.mux.HandleFunc("GET /auth/grants", h.listGrants)
	h.mux.HandleFunc("DELETE /auth/grants/{id}", h.revokeGrant)

	h.mux.HandleFunc("GET /.well-known/oauth-authorization-server", h.serverMetadata)
	for _, path := range []string{"", "/mcp"} {
		h.mux.Handle("/.well-known/oauth-protected-resource"+path, mcpauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
			Resource:               h.svc.Issuer() + path,
			AuthorizationServers:   []string{h.svc.Issuer()},
			BearerMethodsSupported: []string{"header"},
			ResourceName:           "Jaz CRM",
		}))
	}
	h.mux.HandleFunc("POST /oauth/register", h.register)
	h.mux.HandleFunc("GET /oauth/authorize", h.authorize)
	h.mux.HandleFunc("POST /oauth/authorize", h.decide)
	h.mux.HandleFunc("POST /oauth/token", h.token)
	h.mux.HandleFunc("POST /oauth/revoke", h.revoke)
}

// home tells a signed-in person where agents connect.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	actor, _, err := h.browser(r)
	if err != nil {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}
	user, workspace, err := h.svc.Describe(r.Context(), actor)
	if err != nil {
		h.fail(w, err)
		return
	}
	page(w, http.StatusOK, "Jaz CRM", "Signed in as "+user.Email+" in "+workspace.Name+". Connect agents to "+h.svc.Issuer()+"/mcp.", "")
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	actor, _, ok := h.sessionActor(w, r)
	if !ok {
		return
	}
	user, workspace, err := h.svc.Describe(r.Context(), actor)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name": user.Name, "email": user.Email, "admin": user.Admin,
		"workspace": map[string]string{"id": workspace.ID, "name": workspace.Name},
	})
}

func (h *Handler) signIn(w http.ResponseWriter, r *http.Request, identity signin.Identity) bool {
	user, err := h.members.SignIn(r.Context(), identity)
	if errors.Is(err, workspaces.ErrNotAllowed) || errors.Is(err, workspaces.ErrEmailUnverified) {
		page(w, http.StatusForbidden, "Access denied", identity.Email+": "+err.Error()+".", "/auth/login")
		return false
	}
	if err != nil {
		h.fail(w, err)
		return false
	}
	return h.startSession(w, r, user)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if !isJSON(r) {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "send application/json"})
		return
	}
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := h.svc.EndSession(r.Context(), cookie.Value); err != nil {
			h.fail(w, err)
			return
		}
	}
	http.SetCookie(w, h.cookie(sessionCookie, "", time.Unix(0, 0)))
	w.WriteHeader(http.StatusNoContent)
}

type apiKeyView struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Hint      string    `json:"hint"`
	CreatedAt time.Time `json:"createdAt"`
}

func keyView(k storage.APIKey) apiKeyView {
	return apiKeyView{ID: k.ID, Label: k.Label, Hint: k.Hint, CreatedAt: k.CreatedAt}
}

func (h *Handler) listKeys(w http.ResponseWriter, r *http.Request) {
	actor, _, ok := h.sessionActor(w, r)
	if !ok {
		return
	}
	keys, err := h.svc.APIKeys(r.Context(), actor)
	if err != nil {
		h.fail(w, err)
		return
	}
	out := []apiKeyView{}
	for _, k := range keys {
		out = append(out, keyView(k))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createKey(w http.ResponseWriter, r *http.Request) {
	actor, _, ok := h.sessionActor(w, r)
	if !ok {
		return
	}
	var in struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	key, record, err := h.svc.CreateKey(r.Context(), actor.UserID, in.Label, "")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"key": key, "apiKey": keyView(record)})
}

func (h *Handler) deleteKey(w http.ResponseWriter, r *http.Request) {
	actor, _, ok := h.sessionActor(w, r)
	if !ok {
		return
	}
	h.noContent(w, h.svc.DeleteKey(r.Context(), actor, r.PathValue("id")))
}

func (h *Handler) listGrants(w http.ResponseWriter, r *http.Request) {
	actor, _, ok := h.sessionActor(w, r)
	if !ok {
		return
	}
	grants, err := h.svc.Grants(r.Context(), actor)
	if err != nil {
		h.fail(w, err)
		return
	}
	type grantView struct {
		ID         string    `json:"id"`
		ClientName string    `json:"clientName"`
		CreatedAt  time.Time `json:"createdAt"`
		LastUsedAt time.Time `json:"lastUsedAt"`
	}
	out := []grantView{}
	for _, g := range grants {
		out = append(out, grantView(g))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) revokeGrant(w http.ResponseWriter, r *http.Request) {
	actor, _, ok := h.sessionActor(w, r)
	if !ok {
		return
	}
	h.noContent(w, h.svc.RevokeGrant(r.Context(), actor, r.PathValue("id")))
}

func (h *Handler) noContent(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case err != nil:
		h.fail(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) serverMetadata(w http.ResponseWriter, _ *http.Request) {
	issuer := h.svc.Issuer()
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         issuer,
		"authorization_endpoint":                         issuer + "/oauth/authorize",
		"token_endpoint":                                 issuer + "/oauth/token",
		"registration_endpoint":                          issuer + "/oauth/register",
		"revocation_endpoint":                            issuer + "/oauth/revoke",
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"revocation_endpoint_auth_methods_supported":     []string{"none"},
		"authorization_response_iss_parameter_supported": true,
	})
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var meta oauthex.ClientRegistrationMetadata
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		writeJSON(w, http.StatusBadRequest, auth.OAuthError{Code: "invalid_client_metadata", Description: "body must be JSON client metadata"})
		return
	}
	if meta.TokenEndpointAuthMethod != "" && meta.TokenEndpointAuthMethod != "none" {
		writeJSON(w, http.StatusBadRequest, auth.OAuthError{Code: "invalid_client_metadata", Description: "only public clients (token_endpoint_auth_method none) with PKCE are supported"})
		return
	}
	client, err := h.svc.RegisterClient(r.Context(), meta.ClientName, meta.RedirectURIs)
	var oauthErr auth.OAuthError
	if errors.As(err, &oauthErr) {
		writeJSON(w, http.StatusBadRequest, oauthErr)
		return
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, &oauthex.ClientRegistrationResponse{
		ClientRegistrationMetadata: oauthex.ClientRegistrationMetadata{
			RedirectURIs:            client.RedirectURIs,
			TokenEndpointAuthMethod: "none",
			GrantTypes:              []string{"authorization_code", "refresh_token"},
			ResponseTypes:           []string{"code"},
			ClientName:              client.Name,
		},
		ClientID:         client.ID,
		ClientIDIssuedAt: client.CreatedAt,
	})
}

func authorizeRequest(v url.Values) auth.AuthorizeRequest {
	return auth.AuthorizeRequest{
		ResponseType:        v.Get("response_type"),
		ClientID:            v.Get("client_id"),
		RedirectURI:         v.Get("redirect_uri"),
		CodeChallenge:       v.Get("code_challenge"),
		CodeChallengeMethod: v.Get("code_challenge_method"),
		State:               v.Get("state"),
		Scope:               v.Get("scope"),
		Resource:            v.Get("resource"),
	}
}

// authorize shows the consent screen, sending anyone signed out to sign in first.
func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) {
	req := authorizeRequest(r.URL.Query())
	client, err := h.svc.CheckAuthorize(r.Context(), req)
	if !h.authorizeError(w, r, req, err) {
		return
	}
	actor, cookie, err := h.browser(r)
	if err != nil {
		http.Redirect(w, r, "/auth/login?return_to="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}
	consent(w, h.svc, r, client, req, actor, csrf(cookie))
}

func (h *Handler) decide(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	req := authorizeRequest(r.PostForm)
	_, err := h.svc.CheckAuthorize(r.Context(), req)
	if !h.authorizeError(w, r, req, err) {
		return
	}
	actor, cookie, err := h.browser(r)
	if err != nil || r.PostForm.Get("csrf") != csrf(cookie) {
		page(w, http.StatusForbidden, "Authorization expired", "Please return to the application and start again.", "/")
		return
	}
	if r.PostForm.Get("decision") != "allow" {
		http.Redirect(w, r, h.svc.Redirect(req, url.Values{"error": {"access_denied"}}), http.StatusSeeOther)
		return
	}
	target, err := h.svc.Approve(r.Context(), actor, req)
	if err != nil {
		h.fail(w, err)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// authorizeError reports whether the request may proceed, answering otherwise.
func (h *Handler) authorizeError(w http.ResponseWriter, r *http.Request, req auth.AuthorizeRequest, err error) bool {
	var oauthErr auth.OAuthError
	switch {
	case err == nil:
		return true
	case errors.Is(err, auth.ErrBadRedirect):
		page(w, http.StatusBadRequest, "Unknown application", "This authorization request comes from an unregistered application or redirect address.", "/")
	case errors.As(err, &oauthErr):
		http.Redirect(w, r, h.svc.Redirect(req, url.Values{"error": {oauthErr.Code}, "error_description": {oauthErr.Description}}), http.StatusFound)
	default:
		h.fail(w, err)
	}
	return false
}

func (h *Handler) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, auth.OAuthError{Code: "invalid_request", Description: "body must be form encoded"})
		return
	}
	f := r.PostForm
	res, err := h.svc.Token(r.Context(), auth.TokenRequest{
		GrantType:    f.Get("grant_type"),
		ClientID:     f.Get("client_id"),
		Code:         f.Get("code"),
		RedirectURI:  f.Get("redirect_uri"),
		CodeVerifier: f.Get("code_verifier"),
		RefreshToken: f.Get("refresh_token"),
	})
	var oauthErr auth.OAuthError
	switch {
	case errors.As(err, &oauthErr):
		writeJSON(w, http.StatusBadRequest, oauthErr)
	case err != nil:
		h.fail(w, err)
	default:
		writeJSON(w, http.StatusOK, res)
	}
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Revoke(r.Context(), r.FormValue("token")); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// browser authenticates only by session cookie: granting access to an app is
// a person's decision, not something a token or API key can do for itself.
func (h *Handler) browser(r *http.Request) (auth.Actor, *http.Cookie, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return auth.Actor{}, nil, auth.ErrUnauthenticated
	}
	actor, err := h.svc.Session(r.Context(), cookie.Value)
	return actor, cookie, err
}
