// Package authapi serves sign-in, the OAuth 2.1 authorization server and
// the settings endpoints for API keys and authorized applications.
package authapi

import (
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/gluonfield/jaz-tasks/auth"
)

const sessionCookie = "jc_session"

type Handler struct {
	svc     *auth.Service
	members *workspaces.Service
	public  *url.URL
	logger  *log.Logger
	mux     *http.ServeMux
}

func NewHandler(svc *auth.Service, members *workspaces.Service, signIn signin.Config, logger *log.Logger) (*Handler, error) {
	public, err := url.Parse(svc.Issuer())
	if err != nil {
		return nil, err
	}
	h := &Handler{svc: svc, members: members, public: public, logger: logger.WithPrefix("auth"), mux: http.NewServeMux()}
	signIn.PublicURL = svc.Issuer()
	login, err := signin.NewHandler(signIn, "Jaz CRM", sessionCookie+"_login", h.signIn)
	if err != nil {
		return nil, err
	}
	h.mux.Handle("/login", login)
	h.mux.Handle("/auth/", login)
	h.routes()
	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// Session admits a signed-in browser, putting its actor in the request context.
func (h *Handler) Session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, _, ok := h.sessionActor(w, r)
		if ok {
			next.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), actor)))
		}
	})
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

func isJSON(r *http.Request) bool {
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return media == "application/json"
}
