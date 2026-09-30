package interactions

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/google/uuid"
)

// Entry is a conversation a person or a recorder reports: a call, a meeting
// outside the calendar, or a note.
type Entry struct {
	Kind       string
	Title      string
	At         time.Time
	End        *time.Time
	People     []string
	Records    []string
	Notes      string
	Transcript string
	// ExternalID makes a recorder's repeated report update one interaction.
	ExternalID string
}

// Log files a reported conversation. Its people count as engaged: the CRM
// keeps them unless a person decided otherwise.
func (s *Service) Log(ctx context.Context, actor auth.Actor, source string, e Entry) (Interaction, error) {
	if !slices.Contains([]string{Call, Meeting, Note}, e.Kind) {
		return Interaction{}, errs.Invalidf("kind is call, meeting or note")
	}
	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" && strings.TrimSpace(e.Notes+e.Transcript) == "" {
		return Interaction{}, errs.Invalidf("a logged conversation needs a title, notes or a transcript")
	}
	if e.Title == "" {
		e.Title = strings.ToUpper(e.Kind[:1]) + e.Kind[1:]
	}
	if e.At.IsZero() {
		e.At = time.Now()
	}
	if e.ExternalID == "" {
		e.ExternalID = uuid.NewString()
	}
	labels, err := s.records.Labels(ctx, actor.WorkspaceID, e.Records)
	if err != nil {
		return Interaction{}, err
	}
	for _, id := range e.Records {
		if _, ok := labels[id]; !ok {
			return Interaction{}, errs.Invalidf("no record %q", id)
		}
	}
	known, err := s.Known(ctx, actor.WorkspaceID)
	if err != nil {
		return Interaction{}, err
	}
	i, err := s.store.UpsertInteraction(ctx, storage.NewInteraction{
		WorkspaceID: actor.WorkspaceID, Kind: e.Kind, Source: source, ExternalID: e.ExternalID, UserID: &actor.UserID,
		Title: e.Title, StartedAt: e.At, EndedAt: e.End,
	})
	if err != nil {
		return Interaction{}, err
	}
	for _, raw := range e.People {
		kind, value, err := address(raw)
		if err != nil {
			return Interaction{}, err
		}
		h, err := s.store.UpsertHandle(ctx, known.verdict(kind, value))
		if err != nil {
			return Interaction{}, err
		}
		if err := s.store.AddParticipant(ctx, i.ID, h.ID, "attendee"); err != nil {
			return Interaction{}, err
		}
		if h.Triage == Pending || h.Triage == Skipped && deref(h.DecidedBy) != ByUser {
			if err := s.keep(ctx, h, "", ByEngagement, "you logged a conversation with them"); err != nil {
				return Interaction{}, err
			}
		}
	}
	for kind, text := range map[string]string{"note": e.Notes, "transcript": e.Transcript} {
		if strings.TrimSpace(text) == "" {
			continue
		}
		if err := s.store.UpsertPart(ctx, storage.NewPart{InteractionID: i.ID, Kind: kind, ExternalID: kind, At: e.At, Content: &text}); err != nil {
			return Interaction{}, err
		}
	}
	for _, id := range e.Records {
		if err := s.store.AddLink(ctx, i.ID, id, linkSource(actor)); err != nil {
			return Interaction{}, err
		}
	}
	if err := s.store.Relink(ctx, []string{i.ID}); err != nil {
		return Interaction{}, err
	}
	return s.Get(ctx, actor, i.ID)
}

func linkSource(actor auth.Actor) string {
	if actor.Agent {
		return ByAgent
	}
	return ByUser
}

// Link ties an interaction to a record; triage never removes such a link.
func (s *Service) Link(ctx context.Context, actor auth.Actor, interactionID, recordID string) (Interaction, error) {
	if _, err := s.find(ctx, actor.WorkspaceID, interactionID); err != nil {
		return Interaction{}, err
	}
	labels, err := s.records.Labels(ctx, actor.WorkspaceID, []string{recordID})
	if err != nil {
		return Interaction{}, err
	}
	if _, ok := labels[recordID]; !ok {
		return Interaction{}, errs.Invalidf("no record %q", recordID)
	}
	if err := s.store.AddLink(ctx, interactionID, recordID, linkSource(actor)); err != nil {
		return Interaction{}, err
	}
	return s.Get(ctx, actor, interactionID)
}

// Unlink removes a link; content no record links is forgotten.
func (s *Service) Unlink(ctx context.Context, actor auth.Actor, interactionID, recordID string) error {
	if _, err := s.find(ctx, actor.WorkspaceID, interactionID); err != nil {
		return err
	}
	err := s.store.DeleteLink(ctx, interactionID, recordID)
	if errors.Is(err, storage.ErrNotFound) {
		return errs.Invalidf("the interaction is not linked to record %q", recordID)
	}
	return err
}

// Skip removes an interaction from the CRM for good: its links and content
// go, and sync never brings it back.
func (s *Service) Skip(ctx context.Context, actor auth.Actor, id string) error {
	if _, err := s.find(ctx, actor.WorkspaceID, id); err != nil {
		return err
	}
	return s.store.SkipInteraction(ctx, actor.WorkspaceID, id)
}
