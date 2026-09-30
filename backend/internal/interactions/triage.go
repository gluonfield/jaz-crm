package interactions

import (
	"cmp"
	"context"
	"maps"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/errs"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

// smallConversation is the most participants a conversation can have and
// still count as engaging each of them.
const smallConversation = 10

// assessBatch is how many addresses one classifier call judges.
const assessBatch = 25

// Triage settles what evidence can, strongest first: the workspace's own
// addresses, addresses already on a record, then people someone in the
// workspace wrote to or met, then asks the classifier about one batch of the
// rest.
func (s *Service) Triage(ctx context.Context, workspaceID string) error {
	known, err := s.Known(ctx, workspaceID)
	if err != nil {
		return err
	}
	// Own addresses can become known after their mail arrived, such as a
	// mailbox's aliases.
	if err := s.store.MarkInternal(ctx, workspaceID, slices.Collect(maps.Keys(known.own)), slices.Collect(maps.Keys(known.domains))); err != nil {
		return err
	}
	onRecords, err := s.store.HandlesOnRecords(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, pair := range onRecords {
		if err := s.keepByID(ctx, workspaceID, pair.ID, pair.RecordID, "already a record"); err != nil {
			return err
		}
	}
	engaged, err := s.store.EngagedHandles(ctx, workspaceID, smallConversation)
	if err != nil {
		return err
	}
	for _, id := range engaged {
		if err := s.keepByID(ctx, workspaceID, id, "", "you wrote to or met them"); err != nil {
			return err
		}
	}
	orphans, err := s.store.KeptWithoutPerson(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, h := range orphans {
		if err := s.keep(ctx, h, "", *h.DecidedBy, h.Reason); err != nil {
			return err
		}
	}
	return s.assess(ctx, workspaceID)
}

func (s *Service) keepByID(ctx context.Context, workspaceID, id, personID, reason string) error {
	handles, err := s.store.Handles(ctx, workspaceID, []string{id})
	if err != nil || len(handles) == 0 {
		return err
	}
	h := handles[0]
	by := ByEngagement
	if h.Triage == Kept {
		by = *h.DecidedBy
		reason = h.Reason
	}
	return s.keep(ctx, h, personID, by, reason)
}

// keep marks a handle kept, finding or creating its person, and links its
// conversations.
func (s *Service) keep(ctx context.Context, h storage.Handle, personID, by, reason string) error {
	if personID == "" {
		var err error
		if personID, err = s.person(ctx, h); err != nil {
			return err
		}
	}
	err := s.store.SetTriage(ctx, storage.Verdict{WorkspaceID: h.WorkspaceID, ID: h.ID, Triage: Kept, DecidedBy: &by, Reason: reason, PersonID: &personID})
	if err != nil {
		return err
	}
	return s.relink(ctx, h.ID)
}

func (s *Service) relink(ctx context.Context, handleIDs ...string) error {
	ids, err := s.store.InteractionsOfHandles(ctx, handleIDs)
	if err != nil || len(ids) == 0 {
		return err
	}
	return s.store.Relink(ctx, ids)
}

// person finds or creates the person behind a handle, with the company of a
// work email; sync never overwrites what an agent or person wrote.
func (s *Service) person(ctx context.Context, h storage.Handle) (string, error) {
	actor := auth.Actor{WorkspaceID: h.WorkspaceID}
	set := map[string][]string{"phone_numbers": {h.Value}}
	if h.Kind == "email" {
		set = map[string][]string{"email_addresses": {h.Value}}
		if domain := domainOf(h.Value); !freemail(domain) {
			_, _, err := s.records.Upsert(ctx, actor, records.SourceSync, records.Write{Object: "companies", Set: map[string][]string{
				"domains": {domain}, "name": {companyName(domain)},
			}})
			if err != nil {
				return "", err
			}
			set["company"] = []string{domain}
		}
	}
	if h.Name != "" && !strings.Contains(h.Name, "@") {
		set["name"] = []string{h.Name}
	}
	person, _, err := s.records.Upsert(ctx, actor, records.SourceSync, records.Write{Object: "people", Set: set})
	return person.ID, err
}

// assess asks the classifier about addresses no evidence settled. An address
// it leaves undecided stays pending for a person to judge.
func (s *Service) assess(ctx context.Context, workspaceID string) error {
	if s.classifier == nil {
		return nil
	}
	pending, err := s.store.UnassessedHandles(ctx, workspaceID, assessBatch)
	if err != nil || len(pending) == 0 {
		return err
	}
	workspace, err := s.workspaces.Workspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	candidates := make([]Candidate, len(pending))
	for i, p := range pending {
		candidates[i] = Candidate{Address: p.Value, Name: p.Name, Titles: p.Titles}
	}
	judgements, err := s.classifier.Classify(ctx, workspace.Description, candidates)
	if err != nil {
		return err
	}
	verdicts := map[string]Judgement{}
	for _, j := range judgements {
		verdicts[strings.ToLower(j.Address)] = j
	}
	for _, p := range pending {
		j := verdicts[p.Value]
		handles, err := s.store.Handles(ctx, workspaceID, []string{p.ID})
		if err != nil || len(handles) == 0 {
			return err
		}
		switch j.Verdict {
		case "keep":
			err = s.keep(ctx, handles[0], "", ByAgent, j.Reason)
		case "skip":
			err = s.store.SetTriage(ctx, storage.Verdict{WorkspaceID: workspaceID, ID: p.ID, Triage: Skipped, DecidedBy: ptr(ByAgent), Reason: j.Reason})
		default:
			err = s.store.SetTriage(ctx, storage.Verdict{WorkspaceID: workspaceID, ID: p.ID, Triage: Pending, DecidedBy: ptr(ByAgent), Reason: "undecided: " + j.Reason})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Decision is a person's verdict on addresses and on whole domains; a domain
// verdict also applies to addresses not seen yet.
type Decision struct {
	Addresses []string
	Domains   []string
	Keep      bool
	Reason    string
}

// Decide applies a person's verdict, returning how many addresses it changed.
func (s *Service) Decide(ctx context.Context, actor auth.Actor, d Decision) (int, error) {
	ws := actor.WorkspaceID
	verdict := Skipped
	if d.Keep {
		verdict = Kept
	}
	known, err := s.Known(ctx, ws)
	if err != nil {
		return 0, err
	}
	var handles []storage.Handle
	for _, raw := range d.Addresses {
		kind, value, err := address(raw)
		if err != nil {
			return 0, err
		}
		h, err := s.store.UpsertHandle(ctx, known.verdict(kind, value))
		if err != nil {
			return 0, err
		}
		handles = append(handles, h)
	}
	for _, raw := range d.Domains {
		domain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(raw)), "@")
		if !strings.Contains(domain, ".") || known.domains[domain] {
			return 0, errs.Invalidf("%q is not an outside domain", raw)
		}
		if err := s.store.SetDomainRule(ctx, ws, domain, verdict, d.Reason); err != nil {
			return 0, err
		}
		found, err := s.store.HandlesByDomain(ctx, ws, domain)
		if err != nil {
			return 0, err
		}
		handles = append(handles, found...)
	}
	changed := 0
	for _, h := range handles {
		if h.Triage == Internal {
			continue
		}
		if d.Keep {
			err = s.keep(ctx, h, deref(h.PersonID), ByUser, d.Reason)
		} else {
			err = s.store.SetTriage(ctx, storage.Verdict{WorkspaceID: ws, ID: h.ID, Triage: Skipped, DecidedBy: ptr(ByUser), Reason: d.Reason, PersonID: h.PersonID})
		}
		if err == nil && !d.Keep {
			err = s.relink(ctx, h.ID)
		}
		if err != nil {
			return changed, err
		}
		changed++
	}
	return changed, nil
}

// address normalizes an email or phone number a person typed.
func address(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "@") {
		a, err := mail.ParseAddress(raw)
		if err != nil {
			return "", "", errs.Invalidf("%q is not an email address", raw)
		}
		return "email", strings.ToLower(a.Address), nil
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, raw)
	if len(digits) < 6 {
		return "", "", errs.Invalidf("%q is neither an email address nor a phone number", raw)
	}
	if strings.HasPrefix(raw, "+") {
		digits = "+" + digits
	}
	return "phone", digits, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Contact is an address with its verdict and how much it has been seen.
type Contact struct {
	Address      string    `json:"address"`
	Kind         string    `json:"kind"`
	Name         string    `json:"name,omitempty"`
	Triage       string    `json:"status"`
	DecidedBy    string    `json:"decided_by,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	PersonID     string    `json:"person_id,omitempty"`
	Interactions int       `json:"interactions"`
	LastSeen     time.Time `json:"last_seen"`
	Photo        string    `json:"photo,omitempty"`
}

// Contacts lists addresses with a verdict, most recently seen first.
func (s *Service) Contacts(ctx context.Context, actor auth.Actor, verdict, query string, limit int) ([]Contact, error) {
	if verdict == "" {
		verdict = Pending
	}
	q := storage.HandleQuery{WorkspaceID: actor.WorkspaceID, Triage: verdict, Limit: int32(min(max(limit, 1), 200))}
	if limit == 0 {
		q.Limit = 50
	}
	if query = strings.TrimSpace(query); query != "" {
		q.Query = &query
	}
	rows, err := s.store.ListHandles(ctx, q)
	if err != nil {
		return nil, err
	}
	people := []string{}
	for _, r := range rows {
		if r.Handle.PersonID != nil {
			people = append(people, *r.Handle.PersonID)
		}
	}
	labels, err := s.records.Labels(ctx, actor.WorkspaceID, people)
	out := []Contact{}
	for _, r := range rows {
		h := r.Handle
		out = append(out, Contact{
			Address: h.Value, Kind: h.Kind, Name: cmp.Or(labels[deref(h.PersonID)].Name, h.Name), Triage: h.Triage, DecidedBy: deref(h.DecidedBy), Reason: h.Reason,
			PersonID: deref(h.PersonID), Interactions: int(r.Interactions), LastSeen: r.LastSeen, Photo: h.PhotoURL,
		})
	}
	return out, err
}
