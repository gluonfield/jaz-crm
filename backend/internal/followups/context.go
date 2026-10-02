package followups

import (
	"context"
	"slices"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
)

type Record struct {
	interactions.Ref
	Values map[string][]string `json:"values"`
}

func recordInput(r records.Record) Record {
	out := Record{Ref: interactions.Ref{ID: r.ID, Object: r.Object, Name: value(r, "name")}, Values: map[string][]string{}}
	for _, f := range r.Fields {
		if f.Attribute == "name" {
			continue
		}
		for _, v := range f.Values {
			out.Values[f.Attribute] = append(out.Values[f.Attribute], v.Text)
		}
	}
	return out
}

func (a *Agent) conversation(ctx context.Context, actor auth.Actor, conv interactions.Interaction) (Conversation, error) {
	ws, err := a.workspaces.Workspace(ctx, actor.WorkspaceID)
	if err != nil {
		return Conversation{}, err
	}
	users, err := a.workspaces.Users(ctx, actor.WorkspaceID)
	if err != nil {
		return Conversation{}, err
	}
	stored, err := a.store.Interactions(ctx, actor.WorkspaceID, []string{conv.ID})
	if err != nil {
		return Conversation{}, err
	}
	in := Conversation{Today: time.Now().UTC().Format(time.DateOnly), Purpose: ws.Description, Kind: conv.Kind, Channel: conv.Channel, Title: conv.Title, Description: conv.Invitation, Messages: conversationLines(conv), Records: []Record{}, FollowUps: []Open{}, Contexts: []Context{}}
	for _, u := range users {
		person := Person{Name: u.Name, Address: u.Email}
		in.Us = append(in.Us, person)
		if len(stored) == 1 && stored[0].UserID != nil && *stored[0].UserID == u.ID {
			in.Sender = &person
		}
	}
	for _, p := range conv.Participants {
		in.Participants = append(in.Participants, Person{Name: p.Name, Address: p.Address})
	}
	pending := []string{}
	for _, ref := range conv.Records {
		pending = append(pending, ref.ID)
	}
	seen := map[string]bool{}
	for i := 0; i < len(pending); i++ {
		id := pending[i]
		if seen[id] {
			continue
		}
		seen[id] = true
		r, err := a.crm.Get(ctx, actor, id)
		if err != nil {
			return Conversation{}, err
		}
		in.Records = append(in.Records, recordInput(r))
		if r.Object == "people" {
			in.Contexts = append(in.Contexts, Context{Person: r.ID, Context: value(r, records.ContextAttribute)})
		}
		if r.Object == "people" || r.Object == "deals" {
			pending = append(pending, refs(r, "company")...)
		}
		if attribute := map[string]string{"people": "people", "companies": "company"}[r.Object]; attribute != "" {
			deals, err := a.searchRecords(ctx, actor, records.Search{Object: "deals", Where: map[string]string{attribute: id}})
			if err != nil {
				return Conversation{}, err
			}
			for _, deal := range deals {
				pending = append(pending, deal.ID)
			}
		}
		if attribute := subjects[r.Object]; attribute != "" {
			open, err := a.searchRecords(ctx, actor, records.Search{Object: records.FollowUps, Where: map[string]string{attribute: id, "status": "Open"}})
			if err != nil {
				return Conversation{}, err
			}
			for _, f := range open {
				if !slices.ContainsFunc(in.FollowUps, func(o Open) bool { return o.ID == f.ID }) {
					in.FollowUps = append(in.FollowUps, Open{ID: f.ID, Action: value(f, "name"), WaitingOn: value(f, "waiting_on"), ReviewOn: value(f, "review_on"), Draft: value(f, "draft")})
				}
			}
		}
	}
	in.Company, err = a.companyKnowledge(ctx, actor)
	return in, err
}

func (a *Agent) companyKnowledge(ctx context.Context, actor auth.Actor) ([]Record, error) {
	pages, err := a.searchRecords(ctx, actor, records.Search{Object: records.Pages, Where: map[string]string{"name": "Company"}})
	if err != nil {
		return nil, err
	}
	out := []Record{}
	seen := map[string]bool{}
	for i := 0; i < len(pages); i++ {
		id := pages[i].ID
		if seen[id] {
			continue
		}
		seen[id] = true
		page, err := a.crm.Get(ctx, actor, id)
		if err != nil {
			return nil, err
		}
		out = append(out, recordInput(page))
		children, err := a.searchRecords(ctx, actor, records.Search{Object: records.Pages, Where: map[string]string{"parent": id}})
		if err != nil {
			return nil, err
		}
		pages = append(pages, children...)
	}
	return out, nil
}

func (a *Agent) searchRecords(ctx context.Context, actor auth.Actor, q records.Search) ([]records.Record, error) {
	q.Limit = 100
	out := []records.Record{}
	for {
		page, total, err := a.crm.Search(ctx, actor, q)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		q.Offset += len(page)
		if len(page) == 0 || q.Offset >= total {
			return out, nil
		}
	}
}
