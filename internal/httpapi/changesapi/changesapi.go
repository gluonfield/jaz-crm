// Package changesapi streams server-sent events to a signed-in page, one
// whenever its workspace's data changes.
package changesapi

import (
	"io"
	"net/http"
	"time"

	"github.com/gluonfield/jaz-crm/internal/auth"
	"github.com/gluonfield/jaz-crm/internal/changes"
)

const (
	// gap spaces events, so a burst of writes such as a sync refetches a page
	// at most this often; a burst's first change is sent at once.
	gap = 5 * time.Second
	// heartbeat keeps proxies from closing an idle stream; Cloudflare closes
	// one after 100 seconds without data.
	heartbeat = 30 * time.Second
)

type Handler struct {
	hub *changes.Hub
}

func NewHandler(hub *changes.Hub) *Handler {
	return &Handler{hub: hub}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFrom(r.Context())
	sub := h.hub.Subscribe(actor.WorkspaceID)
	defer sub.Close()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	flush := http.NewResponseController(w).Flush
	send := func(event string) bool {
		_, err := io.WriteString(w, event)
		return err == nil && flush() == nil
	}
	if !send(":\n\n") {
		return
	}
	beat := time.NewTicker(heartbeat)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-beat.C:
			if !send(":\n\n") {
				return
			}
		case _, open := <-sub.C:
			if !open || !send("data: changed\n\n") {
				return
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(gap):
			}
		}
	}
}
