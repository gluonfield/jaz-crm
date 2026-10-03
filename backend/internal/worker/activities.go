package worker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/logos"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// contentBatch is how many message bodies one pass fetches.
const contentBatch = 50

// photoInterval is how often profile pictures are read again.
const photoInterval = 24 * time.Hour

// Config shapes what a sync reaches for.
type Config struct {
	// PubSubTopic, when set, receives Gmail push notifications.
	PubSubTopic string
	// CalendarWebhook, when set, receives Calendar push notifications.
	CalendarWebhook string
}

type Activities struct {
	Connections  *connections.Service
	Interactions *interactions.Service
	Logos        *logos.Service
	Agent        *followups.Agent
	Config       Config
}

func NewActivities(c *connections.Service, i *interactions.Service, l *logos.Service, f *followups.Agent, cfg Config) *Activities {
	return &Activities{Connections: c, Interactions: i, Logos: l, Agent: f, Config: cfg}
}

// MeetingRef is a meeting whose transcript is due.
type MeetingRef struct {
	InteractionID string
	ConnectionID  string
	MeetCode      string
	Start         time.Time
	End           time.Time
}

type session struct {
	conn   storage.Connection
	google *google.Client
	known  interactions.Known
}

// connection loads a connection that is syncing; a deleted or revoked one
// ends the sync.
func (a *Activities) connection(ctx context.Context, id string) (storage.Connection, error) {
	c, err := a.Connections.Connection(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return c, temporal.NewNonRetryableApplicationError("connection deleted", errGone, err)
	}
	if err == nil && c.Status != "active" {
		return c, temporal.NewNonRetryableApplicationError("connection revoked", errRevoked, nil)
	}
	return c, err
}

// session loads a connection with its Google client.
func (a *Activities) session(ctx context.Context, id string) (session, error) {
	c, err := a.connection(ctx, id)
	if err != nil {
		return session{}, err
	}
	g, err := a.Connections.Google(ctx, c)
	if err != nil {
		return session{}, err
	}
	known, err := a.Interactions.Known(ctx, c.WorkspaceID)
	return session{conn: c, google: g, known: known}, err
}

// classify turns a revoked grant into the error that ends the sync.
func classify(err error) error {
	if errors.Is(err, google.ErrRevoked) {
		return temporal.NewNonRetryableApplicationError(err.Error(), errRevoked, err)
	}
	return err
}

// Aliases records the addresses the mailbox sends as, so mail sent from any
// of them is the workspace's own. Addresses it receives at are learned as
// mail arrives.
func (a *Activities) Aliases(ctx context.Context, id string) error {
	s, err := a.session(ctx, id)
	if err != nil {
		return err
	}
	aliases, err := s.google.SendAs(ctx)
	if err != nil {
		return classify(err)
	}
	return a.Connections.AddAliases(ctx, id, aliases)
}

// Photos reads, once a day, the profile pictures of the people the mailbox
// has written to. An account connected before it granted contacts access
// skips pictures until it connects again.
func (a *Activities) Photos(ctx context.Context, id string) error {
	s, err := a.session(ctx, id)
	if err != nil {
		return err
	}
	last, err := a.Connections.Cursor(ctx, id, connections.StreamPhotos)
	if err != nil {
		return err
	}
	if at, err := time.Parse(time.RFC3339, last); err == nil && time.Since(at) < photoInterval {
		return nil
	}
	photos := map[string]string{}
	for token := ""; ; {
		page, next, err := s.google.ContactPhotos(ctx, token)
		var denied *google.APIError
		if errors.As(err, &denied) && denied.Status == http.StatusForbidden {
			return nil
		}
		if err != nil {
			return classify(err)
		}
		maps.Copy(photos, page)
		if token = next; token == "" {
			break
		}
		activity.RecordHeartbeat(ctx, len(photos))
	}
	if err := a.Interactions.SetPhotos(ctx, s.conn.WorkspaceID, photos); err != nil {
		return err
	}
	return a.Connections.SetCursor(ctx, id, connections.StreamPhotos, time.Now().UTC().Format(time.RFC3339))
}

// GmailBackfill ingests one page of the mailbox's history, reporting whether
// the backfill is complete. The history cursor is captured first, so mail
// arriving during the backfill is picked up incrementally.
func (a *Activities) GmailBackfill(ctx context.Context, id string) (bool, error) {
	s, err := a.session(ctx, id)
	if err != nil {
		return false, err
	}
	page, err := a.Connections.Cursor(ctx, id, connections.StreamBackfill)
	if err != nil || page == connections.BackfillDone {
		return page == connections.BackfillDone, err
	}
	history, err := a.Connections.Cursor(ctx, id, connections.StreamHistory)
	if err != nil {
		return false, err
	}
	if history == "" {
		profile, err := s.google.Profile(ctx)
		if err != nil {
			return false, classify(err)
		}
		if err := a.Connections.SetCursor(ctx, id, connections.StreamHistory, profile.HistoryID); err != nil {
			return false, err
		}
	}
	query := "after:" + a.Connections.Since().Format("2006/01/02") + " -in:chats"
	list, err := s.google.ListMessages(ctx, query, page)
	if err != nil {
		return false, classify(err)
	}
	if err := a.ingest(ctx, s, list.IDs); err != nil {
		return false, err
	}
	next := list.Next
	if next == "" {
		next = connections.BackfillDone
	}
	return next == connections.BackfillDone, a.Connections.SetCursor(ctx, id, connections.StreamBackfill, next)
}

func (a *Activities) GmailDrafts(ctx context.Context, id string) error {
	defer heartbeats(ctx)()
	s, err := a.session(ctx, id)
	if err != nil {
		return err
	}
	return classify(a.Agent.SyncGmailDrafts(ctx, s.conn, s.google))
}

// GmailIncremental ingests mail added since the history cursor. A cursor
// too old for Gmail restarts the backfill.
func (a *Activities) GmailIncremental(ctx context.Context, id string) error {
	history, err := a.Connections.Cursor(ctx, id, connections.StreamHistory)
	if err != nil || history == "" {
		return err
	}
	s, err := a.session(ctx, id)
	if err != nil {
		return err
	}
	latest := history
	for token := ""; ; {
		page, err := s.google.History(ctx, history, token)
		if errors.Is(err, google.ErrExpiredCursor) {
			if err := a.Connections.ResetCursor(ctx, id, connections.StreamBackfill); err != nil {
				return err
			}
			return a.Connections.ResetCursor(ctx, id, connections.StreamHistory)
		}
		if err != nil {
			return classify(err)
		}
		if err := a.ingest(ctx, s, page.MessageIDs); err != nil {
			return err
		}
		if page.HistoryID != "" {
			latest = page.HistoryID
		}
		if token = page.Next; token == "" {
			break
		}
	}
	return a.Connections.SetCursor(ctx, id, connections.StreamHistory, latest)
}

// ingest stores messages and learns the mailbox's addresses from where they
// were delivered; list mail is left out, as a list may name itself there.
func (a *Activities) ingest(ctx context.Context, s session, ids []string) error {
	var delivered []string
	for chunk := range slices.Chunk(ids, contentBatch) {
		messages, err := s.google.Messages(ctx, chunk, false)
		if err != nil {
			return classify(err)
		}
		for _, m := range messages {
			if m.ID == "" {
				continue
			}
			if err := a.Interactions.IngestEmail(ctx, s.known, emailOf(s.conn, m)); err != nil {
				return err
			}
			if !m.Bulk {
				delivered = append(delivered, m.DeliveredTo...)
			}
		}
		activity.RecordHeartbeat(ctx, len(delivered))
	}
	return a.Connections.AddAliases(ctx, s.conn.ID, delivered)
}

func emailOf(c storage.Connection, m google.Message) interactions.EmailMessage {
	return interactions.EmailMessage{
		ConnectionID: c.ID, UserID: c.UserID, ProviderID: m.ID, ThreadID: m.ThreadID, MessageID: m.MessageID,
		InReplyTo: m.InReplyTo, References: m.References, From: interactions.Address(m.From),
		To: addresses(m.To), Cc: addresses(m.Cc), Subject: m.Subject, Date: m.Date, Bulk: m.Bulk, Labels: m.Labels,
	}
}

func addresses(in []google.Address) []interactions.Address {
	out := make([]interactions.Address, len(in))
	for i, a := range in {
		out[i] = interactions.Address(a)
	}
	return out
}

// CalendarSync ingests meetings changed since the sync token.
func (a *Activities) CalendarSync(ctx context.Context, id string) error {
	s, err := a.session(ctx, id)
	if err != nil {
		return err
	}
	token, err := a.Connections.Cursor(ctx, id, connections.StreamCalendar)
	if err != nil {
		return err
	}
	for page := ""; ; {
		events, err := s.google.Events(ctx, token, page, a.Connections.Since())
		if errors.Is(err, google.ErrExpiredCursor) {
			token = ""
			page = ""
			continue
		}
		if err != nil {
			return classify(err)
		}
		for _, e := range events.Events {
			if err := a.meeting(ctx, s, e); err != nil {
				return err
			}
		}
		activity.RecordHeartbeat(ctx, page)
		if page = events.Next; page == "" {
			return a.Connections.SetCursor(ctx, id, connections.StreamCalendar, events.SyncToken)
		}
	}
}

// meeting files an event with someone besides the calendar's owner; all-day
// events and solo blocks are not conversations.
func (a *Activities) meeting(ctx context.Context, s session, e google.Event) error {
	if e.Status == "cancelled" {
		return a.Interactions.CancelMeeting(ctx, s.conn.WorkspaceID, e.ID)
	}
	var attendees []interactions.Attendee
	others := 0
	for _, at := range e.Attendees {
		if at.Resource {
			continue
		}
		if !at.Self && at.Email != s.conn.Account {
			others++
		}
		attendees = append(attendees, interactions.Attendee{Email: at.Email, Name: at.Name, Response: at.Response, Organizer: at.Organizer})
	}
	if e.AllDay || others == 0 {
		return nil
	}
	return a.Interactions.IngestMeeting(ctx, s.known, interactions.CalendarEvent{
		ConnectionID: s.conn.ID, UserID: s.conn.UserID, ExternalID: e.ID, Title: e.Summary, Description: e.Description,
		Start: e.Start, End: e.End, MeetCode: e.MeetCode, Attendees: attendees,
	})
}

// Triage settles the connection's workspace's addresses.
func (a *Activities) Triage(ctx context.Context, id string) error {
	c, err := a.connection(ctx, id)
	if err != nil {
		return err
	}
	return a.Interactions.Triage(ctx, c.WorkspaceID)
}

// CompanyLogos looks up the logos of the workspace's new company domains.
func (a *Activities) CompanyLogos(ctx context.Context, id string) error {
	c, err := a.connection(ctx, id)
	if err != nil {
		return err
	}
	return a.Logos.Refresh(ctx, c.WorkspaceID)
}

// FetchContent fetches bodies of linked messages, returning how many.
func (a *Activities) FetchContent(ctx context.Context, id string) (int, error) {
	parts, err := a.Interactions.Unfetched(ctx, id, contentBatch)
	if err != nil || len(parts) == 0 {
		return 0, err
	}
	s, err := a.session(ctx, id)
	if err != nil {
		return 0, err
	}
	ids := make([]string, len(parts))
	for i, p := range parts {
		ids[i] = p.ProviderID
	}
	messages, err := s.google.Messages(ctx, ids, true)
	if err != nil {
		return 0, classify(err)
	}
	for i, p := range parts {
		if slices.Contains(messages[i].Labels, "DRAFT") {
			if err := a.Interactions.IngestEmail(ctx, s.known, emailOf(s.conn, messages[i])); err != nil {
				return 0, err
			}
		}
		if err := a.Interactions.SetContent(ctx, p.ID, messages[i].Text, messages[i].HTML); err != nil {
			return 0, err
		}
	}
	return len(parts), nil
}

// FollowUps keeps the connection's workspace's follow-ups current with its
// changed conversations, returning how many it read. A model call can outlast
// the heartbeat timeout, so it heartbeats on a timer.
func (a *Activities) FollowUps(ctx context.Context, id string) (int, error) {
	c, err := a.connection(ctx, id)
	if err != nil {
		return 0, err
	}
	defer heartbeats(ctx)()
	return a.Agent.Run(ctx, c.WorkspaceID)
}

// Watch renews push notifications a day before they lapse.
func (a *Activities) Watch(ctx context.Context, id string) error {
	if a.Config.PubSubTopic == "" && a.Config.CalendarWebhook == "" {
		return nil
	}
	s, err := a.session(ctx, id)
	if err != nil {
		return err
	}
	soon := time.Now().Add(24 * time.Hour)
	if a.Config.PubSubTopic != "" {
		raw, err := a.Connections.Cursor(ctx, id, connections.StreamWatch)
		if err != nil {
			return err
		}
		if expires, _ := time.Parse(time.RFC3339, raw); expires.Before(soon) {
			_, expires, err := s.google.Watch(ctx, a.Config.PubSubTopic)
			if err != nil {
				return classify(err)
			}
			if err := a.Connections.SetCursor(ctx, id, connections.StreamWatch, expires.Format(time.RFC3339)); err != nil {
				return err
			}
		}
	}
	if a.Config.CalendarWebhook == "" {
		return nil
	}
	old, err := a.Connections.Channel(ctx, id)
	if err != nil || old.Expires.After(soon) {
		return err
	}
	token := make([]byte, 16)
	_, _ = rand.Read(token)
	next := connections.Channel{ID: id + "." + strconv.FormatInt(time.Now().Unix(), 10), Token: hex.EncodeToString(token)}
	next.Resource, next.Expires, err = s.google.WatchEvents(ctx, next.ID, a.Config.CalendarWebhook, next.Token, 7*24*time.Hour)
	if err != nil {
		return classify(err)
	}
	if err := a.Connections.SetChannel(ctx, id, next); err != nil {
		return err
	}
	if old.ID != "" {
		_ = s.google.StopChannel(ctx, old.ID, old.Resource)
	}
	return nil
}

// DueMeetings lists the connection's meetings whose transcript is due.
func (a *Activities) DueMeetings(ctx context.Context, id string) ([]MeetingRef, error) {
	due, err := a.Interactions.DueMeetings(ctx, id)
	out := []MeetingRef{}
	for _, m := range due {
		out = append(out, MeetingRef{InteractionID: m.ID, ConnectionID: id, MeetCode: m.MeetCode, Start: m.StartedAt, End: *m.EndedAt})
	}
	return out, err
}

// FetchTranscript attaches the transcript of the meeting's conference,
// reporting whether one was ready. A recurring meeting reuses its code, so
// only the conference held around the meeting's time counts.
func (a *Activities) FetchTranscript(ctx context.Context, m MeetingRef) (bool, error) {
	s, err := a.session(ctx, m.ConnectionID)
	if err != nil {
		return false, err
	}
	conferences, err := s.google.Conferences(ctx, m.MeetCode)
	if err != nil {
		return false, classify(err)
	}
	var lines []interactions.Line
	for _, c := range conferences {
		if c.Start.Before(m.Start.Add(-2*time.Hour)) || c.Start.After(m.End.Add(2*time.Hour)) {
			continue
		}
		found, err := a.transcript(ctx, s.google, c.Name)
		if err != nil {
			return false, classify(err)
		}
		lines = append(lines, found...)
	}
	if len(lines) == 0 {
		return false, nil
	}
	return true, a.Interactions.AddTranscript(ctx, m.InteractionID, lines)
}

func (a *Activities) transcript(ctx context.Context, g *google.Client, conference string) ([]interactions.Line, error) {
	transcripts, err := g.Transcripts(ctx, conference)
	if err != nil {
		return nil, err
	}
	participants, err := g.Participants(ctx, conference)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, p := range participants {
		names[p.Name] = p.DisplayName
	}
	var lines []interactions.Line
	for _, t := range transcripts {
		if t.State == "STARTED" {
			continue
		}
		entries, err := g.Entries(ctx, t.Name)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			lines = append(lines, interactions.Line{ID: e.Name, Speaker: names[e.Participant], Text: e.Text, At: e.Start})
		}
	}
	return lines, nil
}

func (a *Activities) TranscriptChecked(ctx context.Context, interactionID string) error {
	return a.Interactions.TranscriptChecked(ctx, interactionID)
}

// Revoke marks a connection whose grant Google rejected.
func (a *Activities) Revoke(ctx context.Context, id string) error {
	return a.Connections.Revoke(ctx, id)
}

func heartbeats(ctx context.Context) func() {
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-tick.C:
				activity.RecordHeartbeat(ctx)
			}
		}
	}()
	return func() { close(done) }
}
