package interactions

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

// The views below are also the wire shape of the tools that return them.

type Party struct {
	Address  string `json:"address"`
	Name     string `json:"name,omitempty"`
	Role     string `json:"role"`
	PersonID string `json:"person_id,omitempty"`
	Photo    string `json:"photo,omitempty"`
}

type Ref struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Name   string `json:"name,omitempty"`
}

type MessageView struct {
	At            string   `json:"at"`
	Sender        string   `json:"sender"`
	SenderAddress string   `json:"sender_address,omitempty"`
	Recipients    []string `json:"recipients,omitempty"`
	Direction     string   `json:"direction,omitempty"`
	Text          string   `json:"text"`
	HTML          string   `json:"html,omitempty"`
	Partial       bool     `json:"partial,omitempty"`
}

type Speech struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
	At      string `json:"at,omitempty"`
}

type Drafting struct {
	State     string     `json:"state"`
	Reason    string     `json:"reason,omitempty"`
	StartedAt *time.Time `json:"started_at,omitempty"`
}

type EmailDraft struct {
	FollowUpID string `json:"follow_up_id"`
	Subject    string `json:"subject"`
	State      string `json:"state"`
}

type Interaction struct {
	ID           string        `json:"id"`
	Kind         string        `json:"kind"`
	Channel      string        `json:"channel,omitempty"`
	URL          string        `json:"url,omitempty"`
	Title        string        `json:"title"`
	StartedAt    string        `json:"started_at"`
	EndedAt      *time.Time    `json:"ended_at,omitempty"`
	Participants []Party       `json:"participants"`
	Records      []Ref         `json:"records"`
	Author       string        `json:"author,omitempty"`
	Text         string        `json:"text,omitempty"`
	Invitation   string        `json:"invitation,omitempty"`
	MeetURL      string        `json:"meet_url,omitempty"`
	Transcript   []Speech      `json:"transcript,omitempty"`
	Messages     []MessageView `json:"messages,omitempty"`
	Preview      string        `json:"preview,omitempty"`
	LastMessage  *MessageView  `json:"last_message,omitempty"`
	Drafting     *Drafting     `json:"drafting,omitempty"`
	Drafts       []EmailDraft  `json:"drafts,omitempty"`
}

func limitOf(limit int) int32 {
	if limit <= 0 {
		return 20
	}
	return int32(min(limit, 100))
}

// Timeline lists a record's interactions that started by now, newest first,
// or its upcoming ones, soonest first.
func (s *Service) Timeline(ctx context.Context, actor auth.Actor, recordID string, kinds []string, cursor string, upcoming bool, limit int) ([]Interaction, error) {
	list, err := s.store.Timeline(ctx, storage.TimelineQuery{
		RecordID: recordID, WorkspaceID: actor.WorkspaceID, Kinds: kinds, Cursor: cursor, Upcoming: upcoming, Limit: limitOf(limit),
	})
	if err != nil {
		return nil, err
	}
	return s.views(ctx, actor.WorkspaceID, list, previewView)
}

// Search finds linked interactions by title or content, or lists the latest
// without a query.
func (s *Service) Search(ctx context.Context, actor auth.Actor, query string, limit int) ([]Interaction, error) {
	list, err := s.store.SearchInteractions(ctx, actor.WorkspaceID, strings.TrimSpace(query), limitOf(limit))
	if err != nil {
		return nil, err
	}
	return s.views(ctx, actor.WorkspaceID, list, previewView)
}

// Get shows one interaction in full.
func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (Interaction, error) {
	return s.get(ctx, actor, id, readableView)
}

// Source preserves stored content, including quoted history omitted from display.
func (s *Service) Source(ctx context.Context, actor auth.Actor, id string) (Interaction, error) {
	return s.get(ctx, actor, id, sourceView)
}

func (s *Service) get(ctx context.Context, actor auth.Actor, id string, mode viewMode) (Interaction, error) {
	list, err := s.find(ctx, actor.WorkspaceID, id)
	if err != nil {
		return Interaction{}, err
	}
	views, err := s.views(ctx, actor.WorkspaceID, list, mode)
	if err != nil {
		return Interaction{}, err
	}
	return views[0], nil
}

func (s *Service) find(ctx context.Context, workspaceID, id string) ([]storage.Interaction, error) {
	list, err := s.store.Interactions(ctx, workspaceID, []string{id})
	if err == nil && (len(list) == 0 || list[0].Skipped) {
		err = errs.Invalidf("no interaction %q", id)
	}
	return list, err
}

// Activity is how often a record was in touch up to now, since when, and
// when last, with the start of its latest message and that message's channel.
type Activity struct {
	Interactions int          `json:"interactions"`
	FirstAt      string       `json:"first_at,omitempty"`
	LastAt       string       `json:"last_at,omitempty"`
	Channel      string       `json:"channel,omitempty"`
	LastMessage  *MessageView `json:"last_message,omitempty"`
}

// Activities maps records to their activity; upcoming meetings do not count.
func (s *Service) Activities(ctx context.Context, actor auth.Actor, recordIDs []string) (map[string]Activity, error) {
	rows, err := s.store.RecordActivity(ctx, actor.WorkspaceID, recordIDs)
	if err != nil {
		return nil, err
	}
	latest, err := s.store.RecordLastMessages(ctx, actor.WorkspaceID, recordIDs)
	if err != nil {
		return nil, err
	}
	own, err := s.conns.InternalAddresses(ctx, actor.WorkspaceID)
	if err != nil {
		return nil, err
	}
	people := []string{}
	for _, m := range latest {
		if m.PersonID != nil {
			people = append(people, *m.PersonID)
		}
	}
	labels, err := s.records.Labels(ctx, actor.WorkspaceID, people)
	if err != nil {
		return nil, err
	}
	out := map[string]Activity{}
	for _, r := range rows {
		out[r.RecordID] = Activity{Interactions: int(r.Interactions), FirstAt: formatAt(r.FirstAt, r.FirstDateOnly), LastAt: formatAt(r.LastAt, r.LastDateOnly)}
	}
	for _, m := range latest {
		a := out[m.RecordID]
		text := m.Content
		if m.Channel == "email" {
			text = readable("message", text)
		}
		a.Channel = m.Channel
		a.LastMessage = &MessageView{At: formatAt(m.At, m.DateOnly), Sender: cmp.Or(m.AuthorName, labels[deref(m.PersonID)].Name, m.SenderName, m.SenderAddress), SenderAddress: m.SenderAddress, Direction: direction(m.Channel, m.Direction, m.SenderAddress, own), Text: preview(text)}
		out[m.RecordID] = a
	}
	return out, nil
}

// direction tells our email by its sender's address; other channels keep the
// direction they were logged with.
func direction(channel, logged, sender string, own []string) string {
	if channel != "email" || sender == "" {
		return logged
	}
	if slices.Contains(own, sender) {
		return "sent"
	}
	return "received"
}

type viewMode int

const (
	previewView viewMode = iota
	readableView
	sourceView
)

func (s *Service) views(ctx context.Context, workspaceID string, list []storage.Interaction, mode viewMode) ([]Interaction, error) {
	full := mode != previewView
	ids := make([]string, len(list))
	for i, it := range list {
		ids[i] = it.ID
	}
	participants, err := s.store.Participants(ctx, ids)
	if err != nil {
		return nil, err
	}
	links, err := s.store.Links(ctx, ids)
	if err != nil {
		return nil, err
	}
	parts, err := s.store.Parts(ctx, ids)
	if err != nil {
		return nil, err
	}
	own, err := s.conns.InternalAddresses(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	recordIDs := []string{}
	for _, l := range links {
		recordIDs = append(recordIDs, l.RecordID)
	}
	for _, p := range participants {
		if p.Handle.PersonID != nil {
			recordIDs = append(recordIDs, *p.Handle.PersonID)
		}
	}
	labels, err := s.records.Labels(ctx, workspaceID, recordIDs)
	if err != nil {
		return nil, err
	}
	authors := map[string]Party{}
	out := make([]Interaction, len(list))
	index := map[string]int{}
	for i, it := range list {
		index[it.ID] = i
		out[i] = Interaction{ID: it.ID, Kind: it.Kind, Channel: it.Channel, URL: it.URL, Title: it.Title, StartedAt: formatAt(it.StartedAt, it.DateOnly), EndedAt: it.EndedAt, Participants: []Party{}, Records: []Ref{}}
		if it.DraftingState != "" {
			out[i].Drafting = &Drafting{State: it.DraftingState, Reason: it.DraftingReason, StartedAt: it.DraftingStartedAt}
			if it.DraftingState == "drafting" && it.DraftingStartedAt != nil && time.Since(*it.DraftingStartedAt) > 15*time.Minute {
				out[i].Drafting.State = "failed"
				out[i].Drafting.Reason = "Drafting was interrupted. The system will retry."
			}
		}
		if it.MeetCode != "" {
			out[i].MeetURL = "https://meet.google.com/" + it.MeetCode
		}
	}
	for _, p := range participants {
		h := p.Handle
		name := cmp.Or(labels[deref(h.PersonID)].Name, h.Name)
		party := Party{Address: h.Value, Name: name, Role: p.Role, PersonID: deref(h.PersonID), Photo: h.PhotoURL}
		authors[h.ID] = party
		v := &out[index[p.InteractionID]]
		i := slices.IndexFunc(v.Participants, func(party Party) bool { return party.Address == h.Value })
		if i < 0 {
			v.Participants = append(v.Participants, party)
		} else if slices.Index(roles, p.Role) < slices.Index(roles, v.Participants[i].Role) {
			v.Participants[i].Role = p.Role
		}
	}
	for _, l := range links {
		label := labels[l.RecordID]
		v := &out[index[l.InteractionID]]
		v.Records = append(v.Records, Ref{ID: l.RecordID, Object: label.Object, Name: label.Name})
	}
	if mode != sourceView {
		drafts, err := s.store.InteractionDrafts(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, draft := range drafts {
			if draft.FollowUpID != nil {
				v := &out[index[draft.InteractionID]]
				v.Drafts = append(v.Drafts, EmailDraft{FollowUpID: *draft.FollowUpID, Subject: draft.Subject, State: draft.State})
			}
		}
	}
	for _, p := range parts {
		if p.Kind == "draft" {
			continue
		}
		v := &out[index[p.InteractionID]]
		sender := authors[deref(p.AuthorHandleID)]
		author := cmp.Or(p.AuthorName, sender.Name, sender.Address)
		text := deref(p.Content)
		if mode != sourceView && v.Channel == "email" && p.Kind == "message" {
			text = readable(p.Kind, text)
		}
		switch p.Kind {
		case "message":
			message := MessageView{At: formatAt(*p.At, p.DateOnly), Sender: author, SenderAddress: sender.Address, Recipients: p.Recipients, Direction: direction(v.Channel, p.Direction, sender.Address, own), Text: text, Partial: p.Partial}
			latest := message
			latest.Text = preview(text)
			v.LastMessage, v.Preview = &latest, latest.Text
			if full {
				if mode == readableView && v.Channel == "email" {
					message.HTML = deref(p.HTML)
				}
				v.Messages = append(v.Messages, message)
			}
		case "transcript":
			if full {
				turn := Speech{Speaker: author, Text: text}
				if p.At != nil {
					turn.At = formatAt(*p.At, p.DateOnly)
				}
				v.Transcript = append(v.Transcript, turn)
			}
		case "description":
			if full {
				v.Invitation = text
			}
		case "note":
			if full {
				v.Author = author
				if v.Text != "" {
					v.Text += "\n\n"
				}
				v.Text += text
			}
		}
		if v.Preview == "" && text != "" {
			v.Preview = preview(text)
		}
	}
	return out, nil
}

// roles orders a participant's roles by how much each says; one shows.
var roles = []string{"organizer", "from", "attendee", "to", "cc", "declined"}

// preview is the start of a text, on one line.
func preview(text string) string {
	runes := []rune(strings.Join(strings.Fields(text), " "))
	if len(runes) > 240 {
		return string(runes[:240]) + "…"
	}
	return string(runes)
}

// SetPhotos records profile pictures by email address.
func (s *Service) SetPhotos(ctx context.Context, workspaceID string, photos map[string]string) error {
	return s.store.SetPhotos(ctx, workspaceID, photos)
}

// Photos maps people to a profile picture of one of their addresses.
func (s *Service) Photos(ctx context.Context, actor auth.Actor, personIDs []string) (map[string]string, error) {
	return s.store.PersonPhotos(ctx, actor.WorkspaceID, personIDs)
}
