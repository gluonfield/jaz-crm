// Package webhooks receives pushes: Google's notices that a mailbox or
// calendar changed, and conversations reported by recorders.
package webhooks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
)

// Config verifies Gmail pushes: Pub/Sub signs each with a Google ID token
// for Audience, issued to ServiceAccount.
type Config struct {
	Audience       string
	ServiceAccount string
}

type Handler struct {
	conns  *connections.Service
	syncer connections.Syncer
	convs  *interactions.Service
	keys   *auth.Service
	cfg    Config
	pubsub *oidc.IDTokenVerifier
	logger *log.Logger
}

func NewHandler(conns *connections.Service, syncer connections.Syncer, convs *interactions.Service, keys *auth.Service, cfg Config, logger *log.Logger) *Handler {
	keySet := oidc.NewRemoteKeySet(context.Background(), "https://www.googleapis.com/oauth2/v3/certs")
	return &Handler{
		conns: conns, syncer: syncer, convs: convs, keys: keys, cfg: cfg, logger: logger.WithPrefix("webhooks"),
		pubsub: oidc.NewVerifier("https://accounts.google.com", keySet, &oidc.Config{ClientID: cfg.Audience}),
	}
}

// Gmail wakes the sync of the mailbox a Pub/Sub push names.
func (h *Handler) Gmail(w http.ResponseWriter, r *http.Request) {
	if h.cfg.Audience == "" {
		http.NotFound(w, r)
		return
	}
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	id, err := h.pubsub.Verify(r.Context(), token)
	var claims struct {
		Email string `json:"email"`
	}
	if err == nil {
		err = id.Claims(&claims)
	}
	if err != nil || h.cfg.ServiceAccount != "" && claims.Email != h.cfg.ServiceAccount {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var push struct {
		Message struct {
			Data string `json:"data"`
		} `json:"message"`
	}
	var notice struct {
		EmailAddress string `json:"emailAddress"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&push); err == nil {
		raw, _ := base64.StdEncoding.DecodeString(push.Message.Data)
		_ = json.Unmarshal(raw, &notice)
	}
	active, err := h.conns.Active(r.Context())
	for _, c := range active {
		if strings.EqualFold(c.Account, notice.EmailAddress) {
			err = errors.Join(err, h.syncer.Start(r.Context(), c.ID))
		}
	}
	h.done(w, err)
}

// Calendar wakes the sync whose channel Google named, if the channel's token
// matches.
func (h *Handler) Calendar(w http.ResponseWriter, r *http.Request) {
	channelID := r.Header.Get("X-Goog-Channel-ID")
	connectionID, _, _ := strings.Cut(channelID, ".")
	channel, err := h.conns.Channel(r.Context(), connectionID)
	if err != nil || channel.ID == "" || channel.ID != channelID || channel.Token != r.Header.Get("X-Goog-Channel-Token") {
		http.Error(w, "unknown channel", http.StatusNotFound)
		return
	}
	h.done(w, h.syncer.Start(r.Context(), connectionID))
}

func (h *Handler) done(w http.ResponseWriter, err error) {
	if err != nil {
		h.logger.Error("wake sync", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Interactions files a conversation a recorder reports with an API key.
func (h *Handler) Interactions(w http.ResponseWriter, r *http.Request) {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	actor, err := h.keys.Authenticate(r.Context(), token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var e interactions.Entry
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&e); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	i, err := h.convs.Log(r.Context(), actor, e)
	switch {
	case errors.As(err, new(errs.Invalid)):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case err != nil:
		h.logger.Error("log interaction", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	default:
		writeJSON(w, http.StatusOK, map[string]string{"id": i.ID})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
