package followups

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/log"
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

// Summarizer writes a person's context from scratch, for people with open
// follow-ups whom no conversation has summarised yet.
type Summarizer interface {
	Summarize(ctx context.Context, h History) (string, error)
}

// History is what a summarizer reads: who the person is and their recent
// conversations, newest first.
type History struct {
	Today         string   `json:"today"`
	Us            []Person `json:"us"`
	Name          string   `json:"name"`
	Facts         []string `json:"facts"`
	Conversations []Thread `json:"conversations"`
}

// Thread is one past conversation as a summarizer reads it.
type Thread struct {
	Title     string `json:"title"`
	Channel   string `json:"channel,omitempty"`
	StartedAt string `json:"started_at"`
	Lines     []Line `json:"lines"`
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
	Contexts     []Context          `json:"contexts"`
	// owner is the member whose mailbox the conversation came from.
	owner string
}

// Context is a person's relationship summary: the current one a planner
// reads, or the merged one it returns.
type Context struct {
	Person  string `json:"person"`
	Context string `json:"context"`
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

// Plan is the follow-ups to create or change and the contexts to rewrite;
// an empty one changes nothing.
type Plan struct {
	FollowUps []Change  `json:"follow_ups"`
	Contexts  []Context `json:"contexts"`
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
	summarizer Summarizer
	logger     *log.Logger
}

type AgentParams struct {
	fx.In
	Service      *Service
	Workspaces   storage.WorkspaceStore
	Connections  storage.ConnectionStore
	Interactions *interactions.Service
	Logger       *log.Logger
	// Planner and Summarizer are absent when no model is configured, and the
	// agent idles.
	Planner    Planner    `optional:"true"`
	Summarizer Summarizer `optional:"true"`
}

func NewAgent(p AgentParams) *Agent {
	return &Agent{Service: p.Service, workspaces: p.Workspaces, addresses: p.Connections, convs: p.Interactions, planner: p.Planner, summarizer: p.Summarizer, logger: p.Logger.WithPrefix("follow-ups")}
}

const (
	// batch bounds the conversations one run reads.
	batch = 10
	// window is how recent content must be to act on; older mail is history.
	window = 7 * 24 * time.Hour
	// budget bounds the characters of a conversation a planner reads.
	budget = 24000
)

// Run reads up to a batch of the workspace's conversations whose content
// changed since the agent last read them, newest first, and reports how many
// it read. Each is claimed first, so concurrent runs never read one twice. One
// the model could not plan is left for a later run; one whose plan could not
// be applied is logged and read again only when it changes.
func (a *Agent) Run(ctx context.Context, workspaceID string) (int, error) {
	if a.planner == nil {
		return 0, nil
	}
	candidates, err := a.store.FollowUpCandidates(ctx, workspaceID, time.Now().Add(-window), batch)
	if err != nil {
		return 0, err
	}
	read := 0
	for _, c := range candidates {
		claimed, err := a.store.ClaimFollowUp(ctx, c.ID, c.FollowedUpAt, &c.LatestAt)
		if err != nil {
			return read, err
		}
		if !claimed {
			continue
		}
		planned, err := a.follow(ctx, workspaceID, c.ID)
		if !planned {
			if _, err := a.store.ClaimFollowUp(ctx, c.ID, &c.LatestAt, c.FollowedUpAt); err != nil {
				return read, err
			}
		}
		if err != nil {
			a.logger.Warn("conversation not followed up", "interaction", c.ID, "planned", planned, "error", err)
			continue
		}
		read++
	}
	if err := a.backfill(ctx, workspaceID); err != nil {
		a.logger.Warn("contexts not written", "workspace", workspaceID, "error", err)
	}
	return read, nil
}

// backfillBatch bounds the missing contexts one run writes.
const backfillBatch = 3

// backfill writes a context for people with open follow-ups who have none,
// from their recent conversations, so every follow-up shows who it is with.
func (a *Agent) backfill(ctx context.Context, workspaceID string) error {
	if a.summarizer == nil {
		return nil
	}
	actor := auth.Actor{WorkspaceID: workspaceID, Agent: true}
	open, _, err := a.crm.Search(ctx, actor, records.Search{Object: records.FollowUps, Where: map[string]string{"status": "Open"}, Limit: 100})
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	written := 0
	for _, f := range open {
		for _, id := range refs(f, "person") {
			if written == backfillBatch {
				return nil
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			person, err := a.crm.Get(ctx, actor, id)
			if err != nil {
				return err
			}
			if value(person, records.ContextAttribute) != "" {
				continue
			}
			h, err := a.history(ctx, actor, person)
			if err != nil {
				return err
			}
			if len(h.Conversations) == 0 {
				continue
			}
			context, err := a.summarizer.Summarize(ctx, h)
			if err != nil {
				return err
			}
			if strings.TrimSpace(context) == "" {
				continue
			}
			if _, _, err := a.crm.Upsert(ctx, actor, records.SourceAgent, records.Write{Object: "people", RecordID: id, Set: map[string][]string{records.ContextAttribute: {plain(context)}}}); err != nil {
				return err
			}
			written++
		}
	}
	return nil
}

// history gathers who a person is and their latest conversations within the
// reading budget.
func (a *Agent) history(ctx context.Context, actor auth.Actor, person records.Record) (History, error) {
	users, err := a.workspaces.Users(ctx, actor.WorkspaceID)
	if err != nil {
		return History{}, err
	}
	h := History{Today: time.Now().UTC().Format(time.DateOnly), Name: value(person, "name"), Facts: []string{}, Conversations: []Thread{}}
	for _, u := range users {
		h.Us = append(h.Us, Person{Name: u.Name, Address: u.Email})
	}
	for _, f := range person.Fields {
		if f.Attribute == "name" || f.Attribute == records.ContextAttribute {
			continue
		}
		var texts []string
		for _, v := range f.Values {
			texts = append(texts, v.Text)
		}
		h.Facts = append(h.Facts, f.Attribute+": "+strings.Join(texts, ", "))
	}
	list, err := a.convs.Timeline(ctx, actor, person.ID, nil, "", false, 8)
	if err != nil {
		return History{}, err
	}
	left := budget
	for _, item := range list {
		if left <= 0 {
			break
		}
		conv, err := a.convs.Get(ctx, actor, item.ID)
		if err != nil {
			return History{}, err
		}
		lines := latest(conv, left)
		for _, l := range lines {
			left -= len(l.Text)
		}
		if len(lines) > 0 {
			h.Conversations = append(h.Conversations, Thread{Title: conv.Title, Channel: conv.Channel, StartedAt: conv.StartedAt, Lines: lines})
		}
	}
	return h, nil
}

// latest is a conversation's note, messages and speaker turns, keeping the
// newest within limit characters.
func latest(conv interactions.Interaction, limit int) []Line {
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
	var out []Line
	for i := len(lines) - 1; i >= 0 && limit > 0; i-- {
		line := lines[i]
		if len(line.Text) > limit {
			line.Text = line.Text[len(line.Text)-limit:]
		}
		limit -= len(line.Text)
		out = append([]Line{line}, out...)
	}
	return out
}

// subjects maps the objects a follow-up references to its attributes.
var subjects = map[string]string{"people": "person", "companies": "company", "deals": "deal"}

// follow plans a conversation and applies the plan, reporting whether the
// model planned it.
func (a *Agent) follow(ctx context.Context, workspaceID, id string) (bool, error) {
	actor := auth.Actor{WorkspaceID: workspaceID, Agent: true}
	conv, err := a.convs.Get(ctx, actor, id)
	if err != nil {
		return true, err
	}
	in, err := a.conversation(ctx, actor, conv)
	if err != nil {
		return true, err
	}
	plan, err := a.planner.Plan(ctx, in)
	if err != nil {
		return false, err
	}
	for _, change := range plan.FollowUps {
		if err := a.apply(ctx, actor, conv, in, change); err != nil {
			return true, err
		}
	}
	for _, c := range plan.Contexts {
		i := slices.IndexFunc(in.Contexts, func(current Context) bool { return current.Person == c.Person })
		if i < 0 || strings.TrimSpace(c.Context) == "" || strings.TrimSpace(c.Context) == in.Contexts[i].Context {
			continue
		}
		if _, _, err := a.crm.Upsert(ctx, actor, records.SourceAgent, records.Write{Object: "people", RecordID: c.Person, Set: map[string][]string{records.ContextAttribute: {plain(c.Context)}}}); err != nil {
			return true, err
		}
	}
	return true, nil
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
	in.Contexts = []Context{}
	for _, ref := range conv.Records {
		if ref.Object != "people" {
			continue
		}
		person, err := a.crm.Get(ctx, actor, ref.ID)
		if err != nil {
			return Conversation{}, err
		}
		in.Contexts = append(in.Contexts, Context{Person: ref.ID, Context: value(person, records.ContextAttribute)})
	}
	for _, ref := range conv.Records {
		attribute := map[string]string{"people": "people", "companies": "company"}[ref.Object]
		if attribute == "" {
			continue
		}
		deals, _, err := a.crm.Search(ctx, actor, records.Search{Object: "deals", Where: map[string]string{attribute: ref.ID}, Limit: 20})
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
	in.Messages = latest(conv, budget)
	seen := map[string]bool{}
	for _, ref := range in.Records {
		attribute, ok := subjects[ref.Object]
		if !ok {
			continue
		}
		open, _, err := a.crm.Search(ctx, actor, records.Search{Object: records.FollowUps, Where: map[string]string{attribute: ref.ID, "status": "Open"}, Limit: 20})
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
	put("name", plain(c.Action))
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
	remove := map[string][]string{}
	if strings.TrimSpace(c.Reply) != "" && (conv.Channel == "email" || conv.Channel == "linkedin") {
		if err := a.draft(ctx, actor, conv, c.Reply, set); err != nil {
			return err
		}
		if c.ID != "" && set["to"] != nil {
			current, err := a.crm.Get(ctx, actor, c.ID)
			if err != nil {
				return err
			}
			for _, attribute := range []string{"to", "cc"} {
				remove[attribute] = slices.DeleteFunc(values(current, attribute), func(address string) bool { return slices.Contains(set[attribute], address) })
				if len(remove[attribute]) == 0 {
					delete(remove, attribute)
				}
			}
		}
	}
	if c.ID == "" && set["name"] == nil || len(set) == 0 {
		return nil
	}
	f, _, err := a.crm.Upsert(ctx, actor, records.SourceAgent, records.Write{Object: records.FollowUps, RecordID: c.ID, Set: set, Remove: remove})
	if err != nil {
		return err
	}
	return a.store.AddLink(ctx, conv.ID, f.ID, interactions.ByAgent)
}

// draft sets a reply's text and channel; an email reply goes to everyone on
// the latest message, as reply-all, but us.
func (a *Agent) draft(ctx context.Context, actor auth.Actor, conv interactions.Interaction, reply string, set map[string][]string) error {
	set["draft"] = []string{plain(reply)}
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
