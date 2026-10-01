package interactions

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/lithammer/shortuuid/v4"
)

type Entry struct {
	Kind       string     `json:"kind" jsonschema:"note, message, call or meeting"`
	Title      string     `json:"title,omitempty"`
	At         string     `json:"at,omitempty" jsonschema:"original date as YYYY-MM-DD or RFC3339 timestamp; required for messages; now for other kinds when omitted"`
	End        *time.Time `json:"end,omitempty"`
	People     []string   `json:"people,omitempty" jsonschema:"participant email addresses or phone numbers"`
	Records    []string   `json:"records,omitempty" jsonschema:"linked CRM record IDs"`
	Text       string     `json:"text,omitempty" jsonschema:"note body, message text or call/meeting notes; keep import explanations in provenance"`
	Transcript []Speech   `json:"transcript,omitempty" jsonschema:"speaker turns, only for a call or meeting"`
	Channel    string     `json:"channel,omitempty" jsonschema:"message channel, for example linkedin, whatsapp or email"`
	Sender     string     `json:"sender,omitempty" jsonschema:"message sender's name or address"`
	Recipients []string   `json:"recipients,omitempty" jsonschema:"message recipients' names or addresses"`
	Direction  string     `json:"direction,omitempty" jsonschema:"sent or received, relative to the workspace"`
	Partial    bool       `json:"partial,omitempty" jsonschema:"true when only a message preview or excerpt is available"`
	Provenance string     `json:"provenance,omitempty" jsonschema:"source links, capture details or import audit; kept separate from text"`
	ExternalID string     `json:"external_id,omitempty" jsonschema:"stable import ID; repeating it updates the same entry"`
}

func (s *Service) Log(ctx context.Context, actor auth.Actor, source string, e Entry) (Interaction, error) {
	if !slices.Contains([]string{Call, Meeting, Note, Message}, e.Kind) {
		return Interaction{}, errs.Invalidf("kind is note, message, call or meeting")
	}
	e.Title = strings.TrimSpace(e.Title)
	e.Text = strings.TrimSpace(e.Text)
	e.Channel = strings.ToLower(strings.TrimSpace(e.Channel))
	e.Sender = strings.TrimSpace(e.Sender)
	e.Provenance = strings.TrimSpace(e.Provenance)
	e.At = strings.TrimSpace(e.At)
	e.ExternalID = strings.TrimSpace(e.ExternalID)
	if e.Text == "" && len(e.Transcript) == 0 {
		return Interaction{}, errs.Invalidf("an entry needs text or a transcript")
	}
	if len(e.Transcript) > 0 && e.Kind != Call && e.Kind != Meeting {
		return Interaction{}, errs.Invalidf("transcripts belong to calls or meetings")
	}
	if e.Kind == Message && (e.Channel == "" || e.Sender == "" || len(e.Recipients) == 0 || e.Text == "" || e.At == "") {
		return Interaction{}, errs.Invalidf("a message needs channel, sender, recipients, text and original at")
	}
	if e.Kind != Message && (e.Channel != "" || e.Sender != "" || len(e.Recipients) > 0 || e.Direction != "" || e.Partial) {
		return Interaction{}, errs.Invalidf("channel, sender, recipients, direction and partial belong to messages")
	}
	if e.Direction != "" && e.Direction != "sent" && e.Direction != "received" {
		return Interaction{}, errs.Invalidf("direction is sent or received")
	}
	for n, recipient := range e.Recipients {
		e.Recipients[n] = strings.TrimSpace(recipient)
		if e.Recipients[n] == "" {
			return Interaction{}, errs.Invalidf("a recipient cannot be empty")
		}
	}
	at, dateOnly, err := parseAt(e.At)
	if err != nil {
		return Interaction{}, err
	}
	if e.End != nil && e.End.Before(at) {
		return Interaction{}, errs.Invalidf("end precedes at")
	}
	parts := []storage.NewPart{}
	if e.Text != "" {
		author, kind := e.Sender, "message"
		if e.Kind != Message {
			author, kind = "Agent", "note"
			if !actor.Agent {
				user, err := s.workspaces.UserByID(ctx, actor.UserID)
				if err != nil {
					return Interaction{}, err
				}
				author = cmp.Or(user.Name, user.Email)
			}
		}
		parts = append(parts, storage.NewPart{Kind: kind, ExternalID: "body", AuthorName: author, At: &at, DateOnly: dateOnly, Content: &e.Text, Recipients: e.Recipients, Direction: e.Direction, Partial: e.Partial})
	}
	for n, turn := range e.Transcript {
		turn.Speaker = strings.TrimSpace(turn.Speaker)
		turn.Text = strings.TrimSpace(turn.Text)
		if turn.Speaker == "" || turn.Text == "" {
			return Interaction{}, errs.Invalidf("each transcript turn needs a speaker and text")
		}
		turn.At = strings.TrimSpace(turn.At)
		var spoken *time.Time
		day := false
		if turn.At != "" {
			parsed, dateOnly, err := parseAt(turn.At)
			if err != nil {
				return Interaction{}, err
			}
			spoken, day = &parsed, dateOnly
		}
		parts = append(parts, storage.NewPart{Kind: "transcript", ExternalID: "turn-" + strconv.Itoa(n), AuthorName: turn.Speaker, At: spoken, DateOnly: day, Content: &turn.Text, Position: int32(n + 1)})
	}
	if e.Title == "" {
		e.Title = strings.ToUpper(e.Kind[:1]) + e.Kind[1:]
	}
	if e.ExternalID == "" {
		e.ExternalID = shortuuid.New()
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
	var i storage.Interaction
	var engaged []storage.Handle
	err = s.store.Atomically(ctx, func(store storage.InteractionStore) error {
		i, err = store.UpsertInteraction(ctx, storage.NewInteraction{
			WorkspaceID: actor.WorkspaceID, Kind: e.Kind, Source: source, ExternalID: e.ExternalID, UserID: &actor.UserID,
			Title: e.Title, StartedAt: at, EndedAt: e.End, Channel: e.Channel, Provenance: e.Provenance, DateOnly: dateOnly,
		})
		if err != nil {
			return err
		}
		if err := store.ClearParticipants(ctx, i.ID); err != nil {
			return err
		}
		if err := store.ClearParts(ctx, i.ID); err != nil {
			return err
		}
		for _, raw := range e.People {
			kind, value, err := address(raw)
			if err != nil {
				return err
			}
			h, err := store.UpsertHandle(ctx, known.verdict(kind, value))
			if err != nil {
				return err
			}
			if err := store.AddParticipant(ctx, i.ID, h.ID, "attendee"); err != nil {
				return err
			}
			if e.Kind != Note && (h.Triage == Pending || h.Triage == Skipped && deref(h.DecidedBy) != ByUser) {
				engaged = append(engaged, h)
			}
		}
		for _, part := range parts {
			part.InteractionID = i.ID
			if err := store.UpsertPart(ctx, part); err != nil {
				return err
			}
		}
		for _, id := range e.Records {
			if err := store.AddLink(ctx, i.ID, id, linkSource(actor)); err != nil {
				return err
			}
		}
		return store.Relink(ctx, []string{i.ID})
	})
	if err != nil {
		return Interaction{}, err
	}
	for _, h := range engaged {
		if err := s.keepByID(ctx, actor.WorkspaceID, h.ID, "", "you logged a conversation with them"); err != nil {
			return Interaction{}, err
		}
	}
	return s.Get(ctx, actor, i.ID)
}

func parseAt(raw string) (time.Time, bool, error) {
	if raw == "" {
		return time.Now(), false, nil
	}
	if at, err := time.Parse(time.DateOnly, raw); err == nil {
		return at, true, nil
	}
	if at, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return at, false, nil
	}
	return time.Time{}, false, errs.Invalidf("at is YYYY-MM-DD or an RFC3339 timestamp")
}

func formatAt(at time.Time, dateOnly bool) string {
	if dateOnly {
		return at.UTC().Format(time.DateOnly)
	}
	return at.UTC().Format(time.RFC3339Nano)
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
