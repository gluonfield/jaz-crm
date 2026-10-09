package interactions

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/lithammer/shortuuid/v4"
)

type Entry struct {
	InteractionID string     `json:"interaction_id,omitempty" jsonschema:"an existing conversation of the same kind to add to, such as the calendar meeting a transcript belongs to"`
	Kind          string     `json:"kind" jsonschema:"note, message, call or meeting"`
	Channel       string     `json:"channel,omitempty" jsonschema:"a message's channel: email, linkedin, whatsapp, x, telegram or sms"`
	URL           string     `json:"url,omitempty" jsonschema:"link to the original, such as the LinkedIn or X thread or the Granola note; logging the same url again adds to that conversation"`
	Title         string     `json:"title,omitempty"`
	At            string     `json:"at,omitempty" jsonschema:"original date as YYYY-MM-DD or RFC3339 timestamp; required for messages; now for other kinds when omitted"`
	End           *time.Time `json:"end,omitempty"`
	People        []string   `json:"people,omitempty" jsonschema:"participants: email addresses, phone numbers, or profile links such as linkedin.com/in/name, with @name on X or Telegram"`
	Records       []string   `json:"records,omitempty" jsonschema:"linked CRM record IDs"`
	Text          string     `json:"text,omitempty" jsonschema:"note body, message text or call/meeting notes, with any sources they cite"`
	Transcript    []Speech   `json:"transcript,omitempty" jsonschema:"speaker turns, only for a call or meeting; replaces its transcript"`
	Sender        string     `json:"sender,omitempty" jsonschema:"message sender's name or address"`
	Recipients    []string   `json:"recipients,omitempty" jsonschema:"message recipients' names or addresses"`
	Direction     string     `json:"direction,omitempty" jsonschema:"sent or received, relative to the workspace"`
	Partial       bool       `json:"partial,omitempty" jsonschema:"true when only a message preview or excerpt is available"`
	Messages      []Said     `json:"messages,omitempty" jsonschema:"a whole message conversation, oldest first, in place of one message's text, at, sender, recipients, direction and partial; messages already logged are updated, not repeated"`
	ExternalID    string     `json:"external_id,omitempty" jsonschema:"stable import ID for an entry without a url; repeating it updates the same entry"`
}

// Said is one message of a logged conversation.
type Said struct {
	At         string   `json:"at" jsonschema:"original date as YYYY-MM-DD or RFC3339 timestamp"`
	Sender     string   `json:"sender"`
	Recipients []string `json:"recipients"`
	Direction  string   `json:"direction,omitempty" jsonschema:"sent or received, relative to the workspace"`
	Text       string   `json:"text"`
	Partial    bool     `json:"partial,omitempty" jsonschema:"true when only a preview or excerpt is available"`
}

// Log saves a conversation someone reports. Logging adds to a conversation
// found by interaction_id, url or external_id: it keeps the messages and
// participants there and replaces notes or a transcript given anew.
func (s *Service) Log(ctx context.Context, actor auth.Actor, e Entry) (Interaction, error) {
	if !slices.Contains([]string{Call, Meeting, Note, Message}, e.Kind) {
		return Interaction{}, errs.Invalidf("kind is note, message, call or meeting")
	}
	e.InteractionID = strings.TrimSpace(e.InteractionID)
	e.Title = strings.TrimSpace(e.Title)
	e.Text = strings.TrimSpace(e.Text)
	e.Channel = strings.ToLower(strings.TrimSpace(e.Channel))
	e.URL = strings.TrimRight(strings.TrimSpace(e.URL), "/")
	e.Sender = strings.TrimSpace(e.Sender)
	e.At = strings.TrimSpace(e.At)
	e.ExternalID = strings.TrimSpace(e.ExternalID)
	single := e.Text != "" || e.At != "" || e.Sender != "" || len(e.Recipients) > 0 || e.Direction != "" || e.Partial
	if len(e.Messages) > 0 && (e.Kind != Message || single) {
		return Interaction{}, errs.Invalidf("messages hold a whole message conversation, in place of one message's fields")
	}
	if e.Kind == Message && len(e.Messages) == 0 {
		e.Messages = []Said{{At: e.At, Sender: e.Sender, Recipients: e.Recipients, Direction: e.Direction, Text: e.Text, Partial: e.Partial}}
	}
	if e.Kind != Message && e.Text == "" && len(e.Transcript) == 0 {
		return Interaction{}, errs.Invalidf("an entry needs text or a transcript")
	}
	if len(e.Transcript) > 0 && e.Kind != Call && e.Kind != Meeting {
		return Interaction{}, errs.Invalidf("transcripts belong to calls or meetings")
	}
	if e.Kind == Message && records.ChannelName(e.Channel) == "" {
		return Interaction{}, errs.Invalidf("a message's channel is one of %s", strings.ToLower(strings.Join(records.Channels, ", ")))
	}
	if e.Kind != Message && (e.Channel != "" || e.Sender != "" || len(e.Recipients) > 0 || e.Direction != "" || e.Partial) {
		return Interaction{}, errs.Invalidf("channel, sender, recipients, direction and partial belong to messages")
	}
	if link, err := url.Parse(e.URL); e.URL != "" && (err != nil || link.Host == "" || link.Scheme != "http" && link.Scheme != "https") {
		return Interaction{}, errs.Invalidf("url %q is not a web link", e.URL)
	}
	at, dateOnly, err := parseAt(e.At)
	if err != nil {
		return Interaction{}, err
	}
	parts := []storage.NewPart{}
	for n, m := range e.Messages {
		m.At, m.Sender, m.Text = strings.TrimSpace(m.At), strings.TrimSpace(m.Sender), strings.TrimSpace(m.Text)
		if m.Sender == "" || len(m.Recipients) == 0 || m.Text == "" || m.At == "" {
			return Interaction{}, errs.Invalidf("a message needs sender, recipients, text and original at")
		}
		if m.Direction != "" && m.Direction != "sent" && m.Direction != "received" {
			return Interaction{}, errs.Invalidf("direction is sent or received")
		}
		for k, recipient := range m.Recipients {
			m.Recipients[k] = strings.TrimSpace(recipient)
			if m.Recipients[k] == "" {
				return Interaction{}, errs.Invalidf("a recipient cannot be empty")
			}
		}
		sent, day, err := parseAt(m.At)
		if err != nil {
			return Interaction{}, err
		}
		if n == 0 {
			at, dateOnly = sent, day
		}
		parts = append(parts, storage.NewPart{Kind: "message", ExternalID: messageKey(m.Direction, sent, m.Text), AuthorName: m.Sender, At: &sent, DateOnly: day, Content: &m.Text, Recipients: m.Recipients, Direction: m.Direction, Partial: m.Partial})
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
	if e.End != nil && e.End.Before(at) {
		return Interaction{}, errs.Invalidf("end precedes at")
	}
	var people []storage.NewHandle
	for _, raw := range e.People {
		kind, value, err := address(raw, e.Channel)
		if err != nil {
			return Interaction{}, err
		}
		people = append(people, storage.NewHandle{Kind: kind, Value: value})
	}
	author := "Agent"
	if !actor.Agent {
		user, err := s.workspaces.UserByID(ctx, actor.UserID)
		if err != nil {
			return Interaction{}, err
		}
		author = cmp.Or(user.Name, user.Email)
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
	key := cmp.Or(e.URL, e.ExternalID)
	var i storage.Interaction
	var engaged []storage.Handle
	err = s.store.Atomically(ctx, func(store storage.InteractionStore) error {
		if i, err = existing(ctx, store, actor.WorkspaceID, e, key); err != nil {
			return err
		}
		if i.ID != "" && e.At == "" {
			at, dateOnly = i.StartedAt, i.DateOnly
		}
		if i.ConnectionID == nil {
			i, err = store.UpsertInteraction(ctx, storage.NewInteraction{
				WorkspaceID: actor.WorkspaceID, Kind: e.Kind, Channel: e.Channel, ExternalID: cmp.Or(i.ExternalID, key, shortuuid.New()), UserID: &actor.UserID,
				Title: cmp.Or(e.Title, i.Title, strings.ToUpper(e.Kind[:1])+e.Kind[1:]), StartedAt: at, EndedAt: cmp.Or(e.End, i.EndedAt), URL: cmp.Or(e.URL, i.URL), DateOnly: dateOnly,
			})
			if err != nil {
				return err
			}
		}
		for _, p := range people {
			h, err := store.UpsertHandle(ctx, known.verdict(p.Kind, p.Value))
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
		if len(e.Transcript) > 0 {
			if err := store.ClearParts(ctx, i.ID, "transcript"); err != nil {
				return err
			}
		}
		if e.Kind != Message && e.Text != "" {
			parts = append(parts, storage.NewPart{Kind: "note", ExternalID: "body", AuthorName: author, At: &at, DateOnly: dateOnly, Content: &e.Text})
		}
		for _, part := range parts {
			part.InteractionID = i.ID
			if err := store.UpsertPart(ctx, part); err != nil {
				return err
			}
		}
		if e.Kind == Message {
			if err := store.SpanMessages(ctx, i.ID); err != nil {
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
	if err := s.keepLogged(ctx, actor.WorkspaceID, engaged); err != nil {
		return Interaction{}, err
	}
	return s.Get(ctx, actor, i.ID)
}

// keepLogged keeps the people someone logged a conversation with. Each joins
// the person holding their address or link; an email address or phone number
// nobody holds becomes a new person, while a link nobody holds waits in
// triage, since a profile alone cannot tell whether that person is already in
// the CRM.
func (s *Service) keepLogged(ctx context.Context, workspaceID string, handles []storage.Handle) error {
	if len(handles) == 0 {
		return nil
	}
	owners, err := s.store.HandlesOnRecords(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, h := range handles {
		i := slices.IndexFunc(owners, func(o storage.HandleRecord) bool { return o.ID == h.ID })
		switch {
		case i >= 0:
			err = s.keepByID(ctx, workspaceID, h.ID, owners[i].RecordID, "you logged a conversation with them")
		case h.Kind == "email" || h.Kind == "phone":
			err = s.keepByID(ctx, workspaceID, h.ID, "", "you logged a conversation with them")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// existing finds the conversation an entry adds to, by its id or by its
// channel and key, or none. A synced conversation takes notes and a
// transcript; its messages come from its account.
func existing(ctx context.Context, store storage.InteractionStore, workspaceID string, e Entry, key string) (storage.Interaction, error) {
	id := e.InteractionID
	if id == "" && key != "" {
		found, err := store.InteractionByExternalID(ctx, workspaceID, e.Channel, key)
		if errors.Is(err, storage.ErrNotFound) {
			return storage.Interaction{}, nil
		}
		if err != nil {
			return storage.Interaction{}, err
		}
		id = found
	}
	if id == "" {
		return storage.Interaction{}, nil
	}
	found, err := store.Interactions(ctx, workspaceID, []string{id})
	switch {
	case err != nil:
		return storage.Interaction{}, err
	case len(found) == 0 || found[0].Skipped:
		return storage.Interaction{}, errs.Invalidf("no interaction %q", id)
	case found[0].Kind != e.Kind || found[0].Channel != e.Channel:
		return storage.Interaction{}, errs.Invalidf("interaction %q is a %s, not a %s", id, strings.TrimSpace(found[0].Channel+" "+found[0].Kind), strings.TrimSpace(e.Channel+" "+e.Kind))
	case found[0].ConnectionID != nil && e.Kind == Message:
		return storage.Interaction{}, errs.Invalidf("interaction %q syncs its messages from its account", id)
	}
	return found[0], nil
}

// messageKey identifies a logged message by its side, day and opening words,
// so logging a conversation again updates the messages it holds, and a full
// message replaces its preview.
func messageKey(direction string, at time.Time, text string) string {
	opening := []rune(strings.Join(strings.Fields(text), " "))
	sum := sha256.Sum256([]byte(direction + "\n" + at.UTC().Format(time.DateOnly) + "\n" + string(opening[:min(len(opening), 32)])))
	return "message:" + hex.EncodeToString(sum[:16])
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
