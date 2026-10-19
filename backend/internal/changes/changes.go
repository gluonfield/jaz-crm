// Package changes tells open pages when their workspace's data changed, so
// they refetch what they show instead of polling.
package changes

import (
	"context"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"go.uber.org/fx"
)

// Hub fans the change feed out to subscribers of each workspace.
type Hub struct {
	feed   storage.ChangeFeed
	logger *log.Logger
	stop   context.CancelFunc
	mu     sync.Mutex
	subs   map[string]map[*Subscription]struct{}
	closed bool
}

func NewHub(lc fx.Lifecycle, feed storage.ChangeFeed, logger *log.Logger) *Hub {
	ctx, stop := context.WithCancel(context.Background())
	h := &Hub{feed: feed, logger: logger.WithPrefix("changes"), stop: stop, subs: map[string]map[*Subscription]struct{}{}}
	// The server starts serving once the hub hears changes.
	lc.Append(fx.StartHook(func(start context.Context) error {
		listening := make(chan struct{})
		go h.run(ctx, sync.OnceFunc(func() { close(listening) }))
		select {
		case <-listening:
			return nil
		case <-start.Done():
			stop()
			return start.Err()
		}
	}))
	return h
}

func (h *Hub) run(ctx context.Context, listening func()) {
	for {
		err := h.feed.Listen(ctx, func() {
			h.signalAll()
			listening()
		}, h.publish)
		if ctx.Err() != nil {
			return
		}
		h.logger.Warn("change feed stopped; reconnecting", "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// signalAll tells every subscriber something may have changed, as the feed
// starts listening again after missing whatever changed meanwhile.
func (h *Hub) signalAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, subs := range h.subs {
		for sub := range subs {
			sub.signal()
		}
	}
}

func (h *Hub) publish(workspaceID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subs[workspaceID] {
		sub.signal()
	}
}

// Close stops listening and ends every subscription, as the server shuts
// down.
func (h *Hub) Close() {
	h.stop()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for _, subs := range h.subs {
		for sub := range subs {
			close(sub.C)
		}
	}
	clear(h.subs)
}

// Subscription follows one workspace's changes.
type Subscription struct {
	// C receives after each change; changes the reader has not taken yet
	// coalesce. It closes with the hub.
	C         chan struct{}
	hub       *Hub
	workspace string
}

func (h *Hub) Subscribe(workspaceID string) *Subscription {
	sub := &Subscription{C: make(chan struct{}, 1), hub: h, workspace: workspaceID}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(sub.C)
		return sub
	}
	if h.subs[workspaceID] == nil {
		h.subs[workspaceID] = map[*Subscription]struct{}{}
	}
	h.subs[workspaceID][sub] = struct{}{}
	return sub
}

func (s *Subscription) Close() {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	delete(s.hub.subs[s.workspace], s)
	if len(s.hub.subs[s.workspace]) == 0 {
		delete(s.hub.subs, s.workspace)
	}
}

func (s *Subscription) signal() {
	select {
	case s.C <- struct{}{}:
	default:
	}
}
