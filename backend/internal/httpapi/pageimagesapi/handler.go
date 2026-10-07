package pageimagesapi

import (
	"errors"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

type Handler struct {
	records *records.Service
	logger  *log.Logger
}

func NewHandler(records *records.Service, logger *log.Logger) *Handler {
	return &Handler{records: records, logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, err := h.records.PageImage(r.Context(), r.PathValue("token"))
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.logger.Error("page image", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(data)
}
