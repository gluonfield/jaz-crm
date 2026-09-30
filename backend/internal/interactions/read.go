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

type Piece struct {
	Kind    string    `json:"kind"`
	At      time.Time `json:"at"`
	Author  string    `json:"author,omitempty"`
	Content string    `json:"content"`
}

type Interaction struct {
	ID           string     `json:"id"`
	Kind         string     `json:"kind"`
	Source       string     `json:"source"`
	Title        string     `json:"title"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	Participants []Party    `json:"participants"`
	Records      []Ref      `json:"records"`
	// Preview opens a list entry; Parts fill a full view.
	Preview string  `json:"preview,omitempty"`
	Parts   []Piece `json:"parts,omitempty"`
}

func limitOf(limit int) int32 {
	if limit <= 0 {
		return 20
	}
	return int32(min(limit, 100))
}

// Timeline lists a record's interactions, newest first.
func (s *Service) Timeline(ctx context.Context, actor auth.Actor, recordID string, kinds []string, before *time.Time, limit int) ([]Interaction, error) {
	list, err := s.store.Timeline(ctx, storage.TimelineQuery{
		RecordID: recordID, WorkspaceID: actor.WorkspaceID, Kinds: append([]string{}, kinds...), Before: before, Limit: limitOf(limit),
	})
	if err != nil {
		return nil, err
	}
	return s.views(ctx, actor.WorkspaceID, list, false)
}

// Search finds linked interactions by title or content, or lists the latest
// without a query.
func (s *Service) Search(ctx context.Context, actor auth.Actor, query string, limit int) ([]Interaction, error) {
	list, err := s.store.SearchInteractions(ctx, actor.WorkspaceID, strings.TrimSpace(query), limitOf(limit))
	if err != nil {
		return nil, err
	}
	return s.views(ctx, actor.WorkspaceID, list, false)
}

// Get shows one interaction in full.
func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (Interaction, error) {
	list, err := s.find(ctx, actor.WorkspaceID, id)
	if err != nil {
		return Interaction{}, err
	}
	views, err := s.views(ctx, actor.WorkspaceID, list, true)
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

// Activity is how often, and when last, a record was in touch.
type Activity struct {
	Interactions int
	LastAt       *time.Time
}

func (s *Service) Activity(ctx context.Context, actor auth.Actor, recordID string) (Activity, error) {
	rows, err := s.store.RecordActivity(ctx, actor.WorkspaceID, []string{recordID})
	if err != nil || len(rows) == 0 {
		return Activity{}, err
	}
	return Activity{Interactions: int(rows[0].Interactions), LastAt: &rows[0].LastAt}, nil
}

func (s *Service) views(ctx context.Context, workspaceID string, list []storage.Interaction, full bool) ([]Interaction, error) {
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
	authors := map[string]string{}
	out := make([]Interaction, len(list))
	index := map[string]int{}
	for i, it := range list {
		index[it.ID] = i
		out[i] = Interaction{ID: it.ID, Kind: it.Kind, Source: it.Source, Title: it.Title, StartedAt: it.StartedAt, EndedAt: it.EndedAt, Participants: []Party{}, Records: []Ref{}}
	}
	for _, p := range participants {
		h := p.Handle
		name := cmp.Or(labels[deref(h.PersonID)].Name, h.Name)
		authors[h.ID] = cmp.Or(name, h.Value)
		v := &out[index[p.InteractionID]]
		i := slices.IndexFunc(v.Participants, func(party Party) bool { return party.Address == h.Value })
		if i < 0 {
			v.Participants = append(v.Participants, Party{Address: h.Value, Name: name, Role: p.Role, PersonID: deref(h.PersonID), Photo: h.PhotoURL})
		} else if slices.Index(roles, p.Role) < slices.Index(roles, v.Participants[i].Role) {
			v.Participants[i].Role = p.Role
		}
	}
	for _, l := range links {
		label := labels[l.RecordID]
		v := &out[index[l.InteractionID]]
		v.Records = append(v.Records, Ref{ID: l.RecordID, Object: label.Object, Name: label.Name})
	}
	for _, p := range parts {
		v := &out[index[p.InteractionID]]
		author := p.AuthorName
		if p.AuthorHandleID != nil && author == "" {
			author = authors[*p.AuthorHandleID]
		}
		content := readable(p.Kind, deref(p.Content))
		if v.Preview == "" && content != "" {
			v.Preview = preview(content)
		}
		if full {
			v.Parts = append(v.Parts, Piece{Kind: p.Kind, At: p.At, Author: author, Content: content})
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
