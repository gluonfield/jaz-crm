// Package authapi serves sign-in, the OAuth 2.1 authorization server and
// the settings endpoints for API keys and authorized applications.
package authapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

const sessionCookie = "jc_session"

type Handler struct {
	svc     *auth.Service
	members *workspaces.Service
	oidc    *auth.OIDC
	public  *url.URL
	logger  *log.Logger
	mux     *http.ServeMux
}

func NewHandler(svc *auth.Service, members *workspaces.Service, oidc *auth.OIDC, logger *log.Logger) (*Handler, error) {
	public, err := url.Parse(svc.Issuer())
	if err != nil {
		return nil, err
	}
	h := &Handler{svc: svc, members: members, oidc: oidc, public: public, logger: logger.WithPrefix("auth"), mux: http.NewServeMux()}
	h.routes()
	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) cookie(name, value string, expires time.Time) *http.Cookie {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.public.Scheme == "https",
	}
	return c
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request, user storage.User) bool {
	token, expires, err := h.svc.CreateSession(r.Context(), user.ID)
	if err != nil {
		h.fail(w, err)
		return false
	}
	http.SetCookie(w, h.cookie(sessionCookie, token, expires))
	return true
}

// sessionActor requires a browser session; bearer tokens cannot manage keys.
// Changes must come as JSON, which a cross-site form cannot send without a
// CORS preflight this server never grants.
func (h *Handler) sessionActor(w http.ResponseWriter, r *http.Request) (auth.Actor, string, bool) {
	if r.Method != http.MethodGet && !isJSON(r) {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "send application/json"})
		return auth.Actor{}, "", false
	}
	cookie, err := r.Cookie(sessionCookie)
	if err == nil {
		if actor, err := h.svc.Session(r.Context(), cookie.Value); err == nil {
			return actor, cookie.Value, true
		}
	}
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "sign in required"})
	return auth.Actor{}, "", false
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	h.logger.Error("request failed", "error", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// returnTo keeps post-login redirects on this site: a path without scheme,
// host, backslashes or control characters, which browsers could read as
// another origin.
func returnTo(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") ||
		strings.ContainsFunc(raw, func(r rune) bool { return r == '\\' || unicode.IsControl(r) }) {
		return "/"
	}
	return raw
}

func random() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func isJSON(r *http.Request) bool {
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return media == "application/json"
}
