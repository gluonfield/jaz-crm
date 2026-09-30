package interactions

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

type Address struct {
	Name  string
	Email string
}

// EmailMessage is one message a connection's mailbox holds.
type EmailMessage struct {
	ConnectionID string
	UserID       string
	ProviderID   string
	ThreadID     string
	MessageID    string
	InReplyTo    string
	References   []string
	From         Address
	To           []Address
	Cc           []Address
	Subject      string
	Date         time.Time
	Bulk         bool
	Labels       []string
}

// IngestEmail files a message's metadata under its thread. Threads merge
// across mailboxes by Message-ID, since each mailbox has its own thread ids.
func (s *Service) IngestEmail(ctx context.Context, known Known, m EmailMessage) error {
	if m.From.Email == "" {
		return nil
	}
	author, err := s.handle(ctx, known, "email", m.From)
	if err != nil {
		return err
	}
	if author.Triage == Pending && (m.Bulk || noise(m.Labels)) {
		if err := s.store.SkipPendingHandle(ctx, author.ID, "bulk or promotional mail"); err != nil {
			return err
		}
	}
	id, err := s.thread(ctx, known.WorkspaceID, m)
	if err != nil {
		return err
	}
	roles := map[string][]Address{"from": {m.From}, "to": m.To, "cc": m.Cc}
	for role, addresses := range roles {
		for _, a := range addresses {
			h, err := s.handle(ctx, known, "email", a)
			if err != nil {
				return err
			}
			if err := s.store.AddParticipant(ctx, id, h.ID, role); err != nil {
				return err
			}
		}
	}
	external := m.MessageID
	if external == "" {
		external = "gmail:" + m.ProviderID
	}
	err = s.store.UpsertPart(ctx, storage.NewPart{
		InteractionID: id, Kind: "message", ExternalID: external, ConnectionID: &m.ConnectionID, ProviderID: &m.ProviderID,
		AuthorHandleID: &author.ID, AuthorName: m.From.Name, At: m.Date,
	})
	if err != nil {
		return err
	}
	return s.store.Relink(ctx, []string{id})
}

func (s *Service) thread(ctx context.Context, workspaceID string, m EmailMessage) (string, error) {
	var ids []string
	for _, id := range append([]string{m.MessageID, m.InReplyTo}, m.References...) {
		if id != "" {
			ids = append(ids, id)
		}
	}
	id, err := s.store.EmailThreadByMessageIDs(ctx, workspaceID, ids)
	if err == nil {
		return id, s.store.ExtendEmailThread(ctx, id, m.Date, subject(m.Subject))
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return "", err
	}
	return s.store.UpsertEmailThread(ctx, storage.EmailThread{
		WorkspaceID: workspaceID, ExternalID: m.ThreadID, ConnectionID: &m.ConnectionID, UserID: &m.UserID, Title: subject(m.Subject), At: m.Date,
	})
}

func (s *Service) handle(ctx context.Context, known Known, kind string, a Address) (storage.Handle, error) {
	h := known.verdict(kind, strings.ToLower(a.Email))
	h.Name = strings.TrimSpace(a.Name)
	return s.store.UpsertHandle(ctx, h)
}

type Attendee struct {
	Email     string
	Name      string
	Response  string
	Organizer bool
}

// CalendarEvent is a meeting on a connection's calendar; ExternalID is the
// provider's event id, which attendees' calendars share.
type CalendarEvent struct {
	ConnectionID string
	UserID       string
	ExternalID   string
	Title        string
	Description  string
	Start        time.Time
	End          time.Time
	MeetCode     string
	Attendees    []Attendee
}

// IngestMeeting files a meeting with its attendees; a declined attendee
// keeps a declined role rather than disappearing.
func (s *Service) IngestMeeting(ctx context.Context, known Known, e CalendarEvent) (storage.Interaction, error) {
	i, err := s.store.UpsertInteraction(ctx, storage.NewInteraction{
		WorkspaceID: known.WorkspaceID, Kind: Meeting, Source: "calendar", ExternalID: e.ExternalID, ConnectionID: &e.ConnectionID,
		UserID: &e.UserID, Title: e.Title, StartedAt: e.Start, EndedAt: &e.End, MeetCode: e.MeetCode,
	})
	if err != nil {
		return i, err
	}
	if err := s.store.ClearParticipants(ctx, i.ID); err != nil {
		return i, err
	}
	for _, a := range e.Attendees {
		h, err := s.handle(ctx, known, "email", Address{Name: a.Name, Email: a.Email})
		if err != nil {
			return i, err
		}
		role := "attendee"
		switch {
		case a.Organizer:
			role = "organizer"
		case a.Response == "declined":
			role = "declined"
		}
		if err := s.store.AddParticipant(ctx, i.ID, h.ID, role); err != nil {
			return i, err
		}
	}
	if e.Description != "" {
		err := s.store.UpsertPart(ctx, storage.NewPart{InteractionID: i.ID, Kind: "description", ExternalID: "description", At: e.Start, Content: &e.Description})
		if err != nil {
			return i, err
		}
	}
	return i, s.store.Relink(ctx, []string{i.ID})
}

// CancelMeeting hides a cancelled meeting, if it was ever seen.
func (s *Service) CancelMeeting(ctx context.Context, workspaceID, externalID string) error {
	id, err := s.store.InteractionByExternalID(ctx, workspaceID, "calendar", externalID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.store.SkipInteraction(ctx, workspaceID, id)
}

// Line is one transcript entry.
type Line struct {
	ID      string
	Speaker string
	Text    string
	At      time.Time
}

// AddTranscript attaches transcript lines to a meeting; lines already there
// are left as they are.
func (s *Service) AddTranscript(ctx context.Context, interactionID string, lines []Line) error {
	for _, l := range lines {
		err := s.store.UpsertPart(ctx, storage.NewPart{
			InteractionID: interactionID, Kind: "transcript", ExternalID: l.ID, AuthorName: l.Speaker, At: l.At, Content: &l.Text,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// DueMeetings are a connection's linked Meet meetings whose transcript is
// worth fetching.
func (s *Service) DueMeetings(ctx context.Context, connectionID string) ([]storage.Interaction, error) {
	return s.store.DueMeetings(ctx, connectionID)
}

// TranscriptChecked records that a meeting's transcript was fetched or given up on.
func (s *Service) TranscriptChecked(ctx context.Context, interactionID string) error {
	return s.store.MarkTranscriptChecked(ctx, interactionID)
}

// Unfetched lists a connection's message parts whose content is due.
func (s *Service) Unfetched(ctx context.Context, connectionID string, limit int32) ([]storage.UnfetchedPart, error) {
	return s.store.UnfetchedParts(ctx, connectionID, limit)
}

func (s *Service) SetContent(ctx context.Context, partID int64, content string) error {
	return s.store.SetPartContent(ctx, partID, content)
}
