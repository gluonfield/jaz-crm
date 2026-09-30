// Package logosapi serves companies' logos by the token record views name
// them with.
package logosapi

import (
	"errors"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/logos"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

type Handler struct {
	logos  *logos.Service
	logger *log.Logger
}

func NewHandler(l *logos.Service, logger *log.Logger) *Handler {
	return &Handler{logos: l, logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logo, err := h.logos.Logo(r.Context(), r.PathValue("token"))
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.logger.Error("logo", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", logo.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// An SVG from another site must not run scripts on this origin when
	// opened directly.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Write(logo.Image)
}
