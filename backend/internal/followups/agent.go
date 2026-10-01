package followups

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"go.uber.org/fx"
)

// Planner reads a conversation that changed and says what it means for the
// follow-ups of the records it concerns.
type Planner interface {
	Plan(ctx context.Context, c Conversation) (Plan, error)
}

// Conversation is what a planner reads: the conversation's latest content,
// who we are, the records it concerns and their open follow-ups.
type Conversation struct {
	Today        string             `json:"today"`
	Purpose      string             `json:"crm_purpose"`
	Us           []Person           `json:"us"`
	Kind         string             `json:"kind"`
	Channel      string             `json:"channel,omitempty"`
	Title        string             `json:"title"`
	Participants []Person           `json:"participants"`
	Messages     []Line             `json:"messages"`
	Records      []interactions.Ref `json:"records"`
	FollowUps    []Open             `json:"open_follow_ups"`
	// owner is the member whose mailbox the conversation came from.
	owner string
}

type Person struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address,omitempty"`
}

// Line is one message, speaker turn or note of a conversation.
type Line struct {
	At        string `json:"at,omitempty"`
	Author    string `json:"author,omitempty"`
	Direction string `json:"direction,omitempty"`
	Text      string `json:"text"`
}

// Open is an open follow-up as a planner sees it.
type Open struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	WaitingOn string `json:"waiting_on,omitempty"`
	ReviewOn  string `json:"review_on,omitempty"`
	Draft     string `json:"draft,omitempty"`
}

// Plan is the follow-ups to create or change; an empty one changes nothing.
type Plan struct {
	FollowUps []Change `json:"follow_ups"`
}

// Change creates a follow-up, or with an id changes an open one. Empty
// fields leave a value as it is; Reply is a draft of our next message.
type Change struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	WaitingOn string `json:"waiting_on"`
	ReviewOn  string `json:"review_on"`
	Status    string `json:"status"`
	Person    string `json:"person"`
	Company   string `json:"company"`
	Deal      string `json:"deal"`
	Reply     string `json:"reply"`
}

// Agent keeps follow-ups current as conversations move.
type Agent struct {
	*Service
	workspaces storage.WorkspaceStore
	addresses  storage.ConnectionStore
	convs      *interactions.Service
	planner    Planner
}

type AgentParams struct {
	fx.In
	Service      *Service
	Workspaces   storage.WorkspaceStore
	Connections  storage.ConnectionStore
	Interactions *interactions.Service
	// Planner is absent when no model is configured, and the agent idles.
	Planner Planner `optional:"true"`
}

func NewAgent(p AgentParams) *Agent {
	return &Agent{Service: p.Service, workspaces: p.Workspaces, addresses: p.Connections, convs: p.Interactions, planner: p.Planner}
}

const (
	// batch bounds the conversations one run reads.
	batch = 10
	// window is how recent content must be to act on; older mail is history.
	window = 7 * 24 * time.Hour
	// budget bounds the characters of a conversation a planner reads.
	budget = 24000
)

// Run reads the workspace's conversations whose content changed since the
// agent last read them, and reports how many it read. Each is claimed first,
// so concurrent runs never read one twice; a failed one is left for the next
// run.
func (a *Agent) Run(ctx context.Context, workspaceID string) (int, error) {
	if a.planner == nil {
		return 0, nil
	}
	candidates, err := a.store.FollowUpCandidates(ctx, workspaceID, time.Now().Add(-window), batch)
	if err != nil {
		return 0, err
	}
	read := 0
	var failed []error
	for _, c := range candidates {
		claimed, err := a.store.ClaimFollowUp(ctx, c.ID, c.FollowedUpAt, &c.LatestAt)
		if err != nil {
			return read, err
		}
		if !claimed {
			continue
		}
		if err := a.follow(ctx, workspaceID, c.ID); err != nil {
			_, undo := a.store.ClaimFollowUp(ctx, c.ID, &c.LatestAt, c.FollowedUpAt)
			failed = append(failed, err, undo)
			continue
		}
		read++
	}
	return read, errors.Join(failed...)
}

// subjects maps the objects a follow-up references to its attributes.
var subjects = map[string]string{"people": "person", "companies": "company", "deals": "deal"}

func (a *Agent) follow(ctx context.Context, workspaceID, id string) error {
	actor := auth.Actor{WorkspaceID: workspaceID, Agent: true}
	conv, err := a.convs.Get(ctx, actor, id)
	if err != nil {
		return err
	}
	in, err := a.conversation(ctx, actor, conv)
	if err != nil {
		return err
	}
	plan, err := a.planner.Plan(ctx, in)
	if err != nil {
		return err
	}
	for _, change := range plan.FollowUps {
		if err := a.apply(ctx, actor, conv, in, change); err != nil {
			return err
		}
	}
	return nil
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
	in := Conversation{Today: time.Now().UTC().Format(time.DateOnly), Purpose: ws.Description, Kind: conv.Kind, Channel: conv.Channel, Title: conv.Title, Records: slices.Clone(conv.Records), FollowUps: []Open{}}
	for _, u := range users {
		in.Us = append(in.Us, Person{Name: u.Name, Address: u.Email})
		if len(stored) == 1 && stored[0].UserID != nil && *stored[0].UserID == u.ID {
			in.owner = u.Email
		}
	}
	for _, ref := range conv.Records {
		attribute := map[string]string{"people": "people", "companies": "company"}[ref.Object]
		if attribute == "" {
			continue
		}
		deals, err := a.crm.Search(ctx, actor, records.Search{Object: "deals", Where: map[string]string{attribute: ref.ID}, Limit: 20})
		if err != nil {
			return Conversation{}, err
		}
		for _, deal := range deals {
			if !slices.ContainsFunc(in.Records, func(r interactions.Ref) bool { return r.ID == deal.ID }) {
				in.Records = append(in.Records, interactions.Ref{ID: deal.ID, Object: "deals", Name: value(deal, "name")})
			}
		}
	}
	for _, p := range conv.Participants {
		in.Participants = append(in.Participants, Person{Name: p.Name, Address: p.Address})
	}
	var lines []Line
	if conv.Text != "" {
		lines = append(lines, Line{At: conv.StartedAt, Author: conv.Author, Text: conv.Text})
	}
	for _, m := range conv.Messages {
		lines = append(lines, Line{At: m.At, Author: m.Sender, Direction: m.Direction, Text: m.Text})
	}
	for _, turn := range conv.Transcript {
		lines = append(lines, Line{At: turn.At, Author: turn.Speaker, Text: turn.Text})
	}
	left := budget
	for i := len(lines) - 1; i >= 0 && left > 0; i-- {
		line := lines[i]
		if len(line.Text) > left {
			line.Text = line.Text[len(line.Text)-left:]
		}
		left -= len(line.Text)
		in.Messages = append([]Line{line}, in.Messages...)
	}
	seen := map[string]bool{}
	for _, ref := range in.Records {
		attribute, ok := subjects[ref.Object]
		if !ok {
			continue
		}
		open, err := a.crm.Search(ctx, actor, records.Search{Object: records.FollowUps, Where: map[string]string{attribute: ref.ID, "status": "Open"}, Limit: 20})
		if err != nil {
			return Conversation{}, err
		}
		for _, f := range open {
			if !seen[f.ID] {
				seen[f.ID] = true
				in.FollowUps = append(in.FollowUps, Open{ID: f.ID, Action: value(f, "name"), WaitingOn: value(f, "waiting_on"), ReviewOn: value(f, "review_on"), Draft: value(f, "draft")})
			}
		}
	}
	return in, nil
}

// apply writes one change as the agent and links the conversation to the
// follow-up. It ignores follow-ups and records the planner was not shown.
func (a *Agent) apply(ctx context.Context, actor auth.Actor, conv interactions.Interaction, in Conversation, c Change) error {
	if c.ID != "" && !slices.ContainsFunc(in.FollowUps, func(o Open) bool { return o.ID == c.ID }) {
		return nil
	}
	set := map[string][]string{}
	put := func(attribute, value string) {
		if value = strings.TrimSpace(value); value != "" {
			set[attribute] = []string{value}
		}
	}
	if c.ID == "" {
		put("owner", in.owner)
	}
	put("name", c.Action)
	put("waiting_on", c.WaitingOn)
	put("status", c.Status)
	if _, err := time.Parse(time.DateOnly, c.ReviewOn); err == nil {
		put("review_on", c.ReviewOn)
	}
	for object, attribute := range subjects {
		id := map[string]string{"person": c.Person, "company": c.Company, "deal": c.Deal}[attribute]
		if slices.ContainsFunc(in.Records, func(r interactions.Ref) bool { return r.ID == id && r.Object == object }) {
			put(attribute, id)
		}
	}
	if strings.TrimSpace(c.Reply) != "" && (conv.Channel == "email" || conv.Channel == "linkedin") {
		if err := a.draft(ctx, actor, conv, c.Reply, set); err != nil {
			return err
		}
	}
	if c.ID == "" && set["name"] == nil || len(set) == 0 {
		return nil
	}
	f, _, err := a.crm.Upsert(ctx, actor, records.SourceAgent, records.Write{Object: records.FollowUps, RecordID: c.ID, Set: set})
	if err != nil {
		return err
	}
	return a.store.AddLink(ctx, conv.ID, f.ID, interactions.ByAgent)
}

// draft sets a reply's text and channel; an email reply goes to everyone on
// the latest message, as reply-all, but us.
func (a *Agent) draft(ctx context.Context, actor auth.Actor, conv interactions.Interaction, reply string, set map[string][]string) error {
	set["draft"] = []string{reply}
	if conv.Channel != "email" {
		set["channel"] = []string{"LinkedIn"}
		return nil
	}
	set["channel"] = []string{"Email"}
	parts, err := a.store.Parts(ctx, []string{conv.ID})
	if err != nil {
		return err
	}
	parts = slices.DeleteFunc(parts, func(p storage.Part) bool { return p.Kind != "message" || p.ConnectionID == nil || p.ProviderID == nil })
	if len(parts) == 0 {
		return nil
	}
	last, _, err := a.original(ctx, parts[len(parts)-1])
	if err != nil {
		return err
	}
	own, err := a.addresses.InternalAddresses(ctx, actor.WorkspaceID)
	if err != nil {
		return err
	}
	answer := last.ReplyTo
	if len(answer) == 0 {
		answer = append(answer, last.From)
	}
	for _, field := range []struct {
		name string
		list []google.Address
	}{{"to", append(answer, last.To...)}, {"cc", last.Cc}} {
		for _, address := range field.list {
			taken := slices.Contains(set["to"], address.Email) || slices.Contains(set["cc"], address.Email)
			if address.Email != "" && !taken && !slices.Contains(own, address.Email) {
				set[field.name] = append(set[field.name], address.Email)
			}
		}
	}
	return nil
}
