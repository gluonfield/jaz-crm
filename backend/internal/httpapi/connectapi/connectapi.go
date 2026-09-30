// Package connectapi runs a signed-in person through Google consent to
// connect their account.
package connectapi

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
)

const cookie = "jc_google"

// done is where the web app shows connections.
const done = "/connections"

type Handler struct {
	conns  *connections.Service
	secure bool
	logger *log.Logger
}

func NewHandler(conns *connections.Service, keys *auth.Service, logger *log.Logger) *Handler {
	return &Handler{conns: conns, secure: strings.HasPrefix(keys.Issuer(), "https://"), logger: logger.WithPrefix("connect")}
}

// Start sends the browser to Google, keeping the state and PKCE verifier in
// a short-lived cookie.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	state := random()
	verifier := random() + random()
	target, err := h.conns.AuthURL(state, verifier)
	if err != nil {
		http.Redirect(w, r, done+"?error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookie, Value: state + "." + verifier, Path: "/connections/google", Expires: time.Now().Add(10 * time.Minute),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: h.secure})
	http.Redirect(w, r, target, http.StatusFound)
}

// Callback finishes consent for the signed-in person's workspace.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(cookie)
	http.SetCookie(w, &http.Cookie{Name: cookie, Path: "/connections/google", MaxAge: -1})
	state, verifier, _ := strings.Cut(valueOf(c, err), ".")
	query := r.URL.Query()
	switch {
	case state == "" || query.Get("state") != state:
		h.finish(w, r, "the connection expired; please try again")
	case query.Get("error") != "":
		h.finish(w, r, "Google reported: "+query.Get("error"))
	default:
		actor, _ := auth.ActorFrom(r.Context())
		_, err := h.conns.Connect(r.Context(), actor, query.Get("code"), verifier)
		if err != nil {
			h.logger.Warn("connect failed", "error", err)
			h.finish(w, r, err.Error())
			return
		}
		h.finish(w, r, "")
	}
}

func (h *Handler) finish(w http.ResponseWriter, r *http.Request, problem string) {
	target := done
	if problem != "" {
		target += "?error=" + url.QueryEscape(problem)
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func valueOf(c *http.Cookie, err error) string {
	if err != nil {
		return ""
	}
	return c.Value
}

func random() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
