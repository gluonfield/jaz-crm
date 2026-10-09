package interactions_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

var ctx = context.Background()

type env struct {
	svc   *interactions.Service
	crm   *records.Service
	store *postgres.Store
	a, b  auth.Actor
	conn  storage.Connection
}

// setup gives two workspaces; the first has a connected mailbox at its own
// company domain, cas.dev.
func setup(t *testing.T, classifier interactions.Classifier) env {
	t.Helper()
	e := setupManual(t, classifier)
	if err := e.store.UpdateTriageSettings(ctx, e.a.WorkspaceID, storage.TriageSettings{AutoKeepEmail: true, AutoKeepMeetings: true, AutoKeepRecords: true, AutoKeepAi: true}); err != nil {
		t.Fatal(err)
	}
	return e
}

func setupManual(t *testing.T, classifier interactions.Classifier) env {
	t.Helper()
	store := postgrestest.New(t)
	people := workspaces.NewService(store, workspaces.Config{})
	var actors []auth.Actor
	for _, email := range []string{"owner@cas.dev", "b@other.dev"} {
		u, err := people.Provision(ctx, email)
		if err != nil {
			t.Fatal(err)
		}
		actors = append(actors, auth.Actor{UserID: u.ID, WorkspaceID: u.WorkspaceID})
	}
	if err := store.UpdateWorkspace(ctx, actors[0].WorkspaceID, storage.WorkspaceUpdate{Name: new("CAS"), Description: new("Manufacturing customers and partners")}); err != nil {
		t.Fatal(err)
	}
	conn, err := store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: actors[0].WorkspaceID, UserID: actors[0].UserID, Provider: "google", Account: "owner@cas.dev", RefreshToken: []byte("sealed")})
	if err != nil {
		t.Fatal(err)
	}
	crm := records.NewService(store)
	svc := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm, Classifier: classifier})
	return env{svc: svc, crm: crm, store: store, a: actors[0], b: actors[1], conn: conn}
}

var start = time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

func message(conn storage.Connection, id, thread, from string, to ...string) interactions.EmailMessage {
	m := interactions.EmailMessage{
		ConnectionID: conn.ID, UserID: conn.UserID, ProviderID: id, ThreadID: thread, MessageID: id + "@mail",
		From: interactions.Address{Email: from}, Subject: "Re: Press line quote", Date: start,
	}
	for _, a := range to {
		m.To = append(m.To, interactions.Address{Email: a})
	}
	return m
}

func (e env) ingest(t *testing.T, messages ...interactions.EmailMessage) {
	t.Helper()
	known, err := e.svc.Known(ctx, e.a.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range messages {
		if err := e.svc.IngestEmail(ctx, known, m); err != nil {
			t.Fatal(err)
		}
	}
}

func (e env) triage(t *testing.T) {
	t.Helper()
	if err := e.svc.Triage(ctx, e.a.WorkspaceID); err != nil {
		t.Fatal(err)
	}
}

func (e env) contacts(t *testing.T, status string) map[string]interactions.Contact {
	t.Helper()
	list, err := e.svc.Contacts(ctx, e.a, status, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]interactions.Contact{}
	for _, c := range list {
		out[c.Address] = c
	}
	return out
}

func (e env) timeline(t *testing.T, recordID string) []interactions.Interaction {
	t.Helper()
	list, err := e.svc.Timeline(ctx, e.a, recordID, nil, "", false, 50)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func values(r records.Record, attr string) []string {
	var out []string
	for _, f := range r.Fields {
		if f.Attribute == attr {
			for _, v := range f.Values {
				out = append(out, v.Text)
			}
		}
	}
	return out
}

// Writing to someone keeps them: they become a person at their company with
// the thread on both timelines. Bulk senders are skipped, colleagues are
// internal, and cold inbound waits for a decision.
func TestTriageKeepsPeopleYouWriteTo(t *testing.T) {
	e := setup(t, nil)
	news := message(e.conn, "n1", "tn", "news@shop.com", "owner@cas.dev")
	news.Bulk = true
	out := message(e.conn, "o1", "t1", "owner@cas.dev", "ada@customer.io")
	reply := message(e.conn, "r1", "t1", "ada@customer.io", "owner@cas.dev")
	reply.From.Name = "Ada Lovelace"
	reply.InReplyTo = "o1@mail"
	reply.Date = start.Add(time.Hour)
	e.ingest(t, news, message(e.conn, "c1", "tc", "bob@supplier.com", "owner@cas.dev"), out, reply, message(e.conn, "p1", "tp", "pat@cas.dev", "owner@cas.dev"))
	e.triage(t)

	if got := e.contacts(t, interactions.Pending); len(got) != 1 || got["bob@supplier.com"].Address == "" {
		t.Fatalf("pending: %v", got)
	}
	if got := e.contacts(t, interactions.Skipped)["news@shop.com"]; got.DecidedBy != interactions.ByRule {
		t.Fatalf("bulk sender: %+v", got)
	}
	if got := e.contacts(t, interactions.Internal); got["pat@cas.dev"].Address == "" || got["owner@cas.dev"].Address == "" {
		t.Fatalf("internal: %v", got)
	}
	ada := e.contacts(t, interactions.Kept)["ada@customer.io"]
	if ada.DecidedBy != interactions.ByEngagement || ada.PersonID == "" {
		t.Fatalf("ada: %+v", ada)
	}
	person, err := e.crm.Get(ctx, e.a, ada.PersonID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(values(person, "name"), []string{"Ada Lovelace"}) || !slices.Equal(values(person, "company"), []string{"Customer"}) {
		t.Fatalf("person: %+v", person)
	}
	thread := e.timeline(t, ada.PersonID)
	if len(thread) != 1 || thread[0].Title != "Press line quote" || !thread[0].EndedAt.Equal(start.Add(time.Hour)) {
		t.Fatalf("timeline: %+v", thread)
	}
	company := person.Fields[slices.IndexFunc(person.Fields, func(f records.Field) bool { return f.Attribute == "company" })].Values[0].RecordID
	if got := e.timeline(t, company); len(got) != 1 || got[0].ID != thread[0].ID {
		t.Fatalf("company timeline: %+v", got)
	}

	// Once someone names the person, that name replaces the one from mail headers.
	if _, _, err := e.crm.Upsert(ctx, e.a, records.SourceUser, records.Write{Object: "people", RecordID: ada.PersonID, Set: map[string][]string{"name": {"Ada King"}}}); err != nil {
		t.Fatal(err)
	}
	if got := e.contacts(t, interactions.Kept)["ada@customer.io"].Name; got != "Ada King" {
		t.Fatalf("contact name: %q", got)
	}
	parties := e.timeline(t, company)[0].Participants
	if i := slices.IndexFunc(parties, func(p interactions.Party) bool { return p.Address == "ada@customer.io" }); i < 0 || parties[i].Name != "Ada King" {
		t.Fatalf("participants: %+v", parties)
	}

	// The same reply in a teammate's mailbox has other ids but one Message-ID.
	other, err := e.store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: e.a.WorkspaceID, UserID: e.a.UserID, Provider: "google", Account: "sales@cas.dev", RefreshToken: []byte("sealed")})
	if err != nil {
		t.Fatal(err)
	}
	copied := reply
	copied.ConnectionID = other.ID
	copied.ProviderID = "x9"
	copied.ThreadID = "their-thread"
	e.ingest(t, copied)
	if got := e.timeline(t, ada.PersonID); len(got) != 1 {
		t.Fatalf("mailboxes did not merge: %+v", got)
	}

	due, err := e.svc.Unfetched(ctx, e.conn.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range due {
		ids = append(ids, p.ProviderID)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []string{"o1", "r1"}) {
		t.Fatalf("content is fetched for linked threads only: %v", ids)
	}
	for _, p := range due {
		content := "Here is the quote."
		if p.ProviderID == "r1" {
			content = "Thanks, that works.\n\nOn Tue, 1 Sep 2026 at 09:00, Owner <owner@cas.dev> wrote:\n> Here is the quote."
		}
		if err := e.svc.SetContent(ctx, p.ID, content, ""); err != nil {
			t.Fatal(err)
		}
	}
	activity, err := e.svc.Activities(ctx, e.a, []string{ada.PersonID})
	want := interactions.MessageView{At: start.Add(time.Hour).Format(time.RFC3339Nano), Sender: "Ada Lovelace", SenderAddress: "ada@customer.io", Direction: "received", Text: "Thanks, that works."}
	if a := activity[ada.PersonID]; err != nil || a.Channel != "email" || !reflect.DeepEqual(a.LastMessage, &want) {
		t.Fatalf("latest message: %+v %+v %v", a, a.LastMessage, err)
	}
}

// A person's decisions are final and domain decisions reach addresses not
// seen yet; skipping someone forgets the content of their conversations.
func TestDecisions(t *testing.T) {
	e := setup(t, nil)
	e.ingest(t, message(e.conn, "c1", "tc", "bob@supplier.com", "owner@cas.dev"))
	if n, err := e.svc.Decide(ctx, e.a, interactions.Decision{Addresses: []string{"Bob <BOB@supplier.com>"}, Keep: true}); err != nil || n != 1 {
		t.Fatalf("keep: %d %v", n, err)
	}
	bob := e.contacts(t, interactions.Kept)["bob@supplier.com"]
	if bob.DecidedBy != interactions.ByUser || len(e.timeline(t, bob.PersonID)) != 1 {
		t.Fatalf("kept bob: %+v", bob)
	}
	due, _ := e.svc.Unfetched(ctx, e.conn.ID, 50)
	if err := e.svc.SetContent(ctx, due[0].ID, "Quote attached.", "<p>Quote attached.</p>"); err != nil {
		t.Fatal(err)
	}
	thread := e.timeline(t, bob.PersonID)[0]

	if _, err := e.svc.Decide(ctx, e.a, interactions.Decision{Domains: []string{"@Supplier.com"}, Keep: false, Reason: "vendor spam"}); err != nil {
		t.Fatal(err)
	}
	if got := e.contacts(t, interactions.Skipped)["bob@supplier.com"]; got.DecidedBy != interactions.ByUser || len(e.timeline(t, bob.PersonID)) != 0 {
		t.Fatalf("skipped bob: %+v", got)
	}
	full, err := e.svc.Get(ctx, e.a, thread.ID)
	if err != nil || full.Messages[0].Text != "" || full.Messages[0].HTML != "" {
		t.Fatalf("content of an unlinked thread must be forgotten: %+v %v", full.Messages, err)
	}
	e.ingest(t, message(e.conn, "c2", "tc2", "carol@supplier.com", "owner@cas.dev"), message(e.conn, "o2", "tc2", "owner@cas.dev", "carol@supplier.com"))
	e.triage(t)
	if got := e.contacts(t, interactions.Skipped)["carol@supplier.com"]; got.Reason != "vendor spam" {
		t.Fatalf("the domain rule must hold against engagement: %+v", got)
	}
	if _, err := e.svc.Decide(ctx, e.a, interactions.Decision{Domains: []string{"cas.dev"}}); err == nil {
		t.Fatal("skipped the workspace's own domain")
	}
	for _, keep := range []bool{false, true} {
		if _, err := e.svc.Decide(ctx, e.a, interactions.Decision{Domains: []string{"Gmail.com"}, Keep: keep}); err == nil {
			t.Fatalf("decided every sender of a webmail domain at once, keep %v", keep)
		}
	}
}

// Deleting a person keeps them out: triage does not create them again.
func TestDeletedPeopleStayDeleted(t *testing.T) {
	e := setup(t, nil)
	e.ingest(t, message(e.conn, "o1", "t1", "owner@cas.dev", "ada@customer.io"))
	e.triage(t)
	ada := e.contacts(t, interactions.Kept)["ada@customer.io"]
	if ada.PersonID == "" {
		t.Fatalf("ada was not kept: %+v", ada)
	}
	if err := e.crm.Delete(ctx, e.b, ada.PersonID); err == nil {
		t.Fatal("deleted another workspace's person")
	}
	if err := e.crm.Delete(ctx, e.a, ada.PersonID); err != nil {
		t.Fatal(err)
	}
	e.ingest(t, message(e.conn, "o2", "t2", "owner@cas.dev", "ada@customer.io"))
	e.triage(t)
	if got := e.contacts(t, interactions.Skipped)["ada@customer.io"]; got.DecidedBy != interactions.ByUser || got.PersonID != "" {
		t.Fatalf("ada after deletion: %+v", got)
	}
	people, _, err := e.crm.Search(ctx, e.a, records.Search{Object: "people"})
	if err != nil || len(people) != 0 {
		t.Fatalf("triage recreated the person: %d %v", len(people), err)
	}
}

// A contact names its company domain only for a work address, so webmail
// is never offered as a domain to skip.
func TestContactsNameWorkDomains(t *testing.T) {
	e := setup(t, nil)
	e.ingest(t, message(e.conn, "c1", "tc", "bob@supplier.com", "owner@cas.dev"), message(e.conn, "c2", "td", "sam@gmail.com", "owner@cas.dev"))
	pending := e.contacts(t, interactions.Pending)
	if pending["bob@supplier.com"].Domain != "supplier.com" || pending["sam@gmail.com"].Domain != "" {
		t.Fatalf("domains: %+v", pending)
	}
}

func TestMeetingsAndTranscripts(t *testing.T) {
	e := setup(t, nil)
	known, err := e.svc.Known(ctx, e.a.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	err = e.svc.IngestMeeting(ctx, known, interactions.CalendarEvent{
		ConnectionID: e.conn.ID, UserID: e.conn.UserID, ExternalID: "ev1", Title: "Line review", Start: now.Add(-2 * time.Hour), End: now.Add(-time.Hour),
		MeetCode: "abc-defg-hij", Attendees: []interactions.Attendee{
			{Email: "owner@cas.dev", Organizer: true, Response: "accepted"},
			{Email: "ada@customer.io", Name: "Ada", Response: "accepted"},
			{Email: "zed@x.io", Response: "declined"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	e.triage(t)
	ada := e.contacts(t, interactions.Kept)["ada@customer.io"]
	if ada.PersonID == "" || e.contacts(t, interactions.Pending)["zed@x.io"].Address == "" {
		t.Fatalf("meeting triage: kept %v", e.contacts(t, interactions.Kept))
	}
	due, err := e.svc.DueMeetings(ctx, e.conn.ID)
	if err != nil || len(due) != 1 || due[0].Title != "Line review" {
		t.Fatalf("due: %+v %v", due, err)
	}
	meeting := due[0]
	if err := e.svc.AddTranscript(ctx, meeting.ID, []interactions.Line{{ID: "e1", Speaker: "Ada", Text: "Can you do 200 a week?", At: now.Add(-2 * time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.TranscriptChecked(ctx, meeting.ID); err != nil {
		t.Fatal(err)
	}
	full, err := e.svc.Get(ctx, e.a, meeting.ID)
	if err != nil || len(full.Transcript) != 1 || full.Transcript[0].Speaker != "Ada" || full.MeetURL != "https://meet.google.com/abc-defg-hij" {
		t.Fatalf("transcript and Meet link: %+v %q %v", full.Transcript, full.MeetURL, err)
	}
	if due, _ := e.svc.DueMeetings(ctx, e.conn.ID); len(due) != 0 {
		t.Fatalf("a checked meeting is due again: %+v", due)
	}
	if err := e.svc.CancelMeeting(ctx, e.a.WorkspaceID, "ev1"); err != nil {
		t.Fatal(err)
	}
	if got := e.timeline(t, ada.PersonID); len(got) != 0 {
		t.Fatalf("a cancelled meeting stays on the timeline: %+v", got)
	}
}

// A record's timeline and activity end at now: scheduled meetings are
// upcoming, soonest first, and never its last contact.
func TestUpcomingMeetings(t *testing.T) {
	e := setup(t, nil)
	known, err := e.svc.Known(ctx, e.a.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Second)
	for id, start := range map[string]time.Time{"later": now.Add(8 * 24 * time.Hour), "next": now.Add(24 * time.Hour), "past": now.Add(-72 * time.Hour)} {
		err := e.svc.IngestMeeting(ctx, known, interactions.CalendarEvent{
			ConnectionID: e.conn.ID, UserID: e.conn.UserID, ExternalID: id, Title: id, Start: start, End: start.Add(time.Hour),
			Attendees: []interactions.Attendee{{Email: "owner@cas.dev", Organizer: true}, {Email: "ada@customer.io"}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	e.triage(t)
	ada := e.contacts(t, interactions.Kept)["ada@customer.io"].PersonID
	if got := e.timeline(t, ada); len(got) != 1 || got[0].Title != "past" {
		t.Fatalf("timeline: %+v", got)
	}
	upcoming, err := e.svc.Timeline(ctx, e.a, ada, nil, "", true, 10)
	if err != nil || len(upcoming) != 2 || upcoming[0].Title != "next" || upcoming[1].Title != "later" {
		t.Fatalf("upcoming: %+v %v", upcoming, err)
	}
	activity, err := e.svc.Activities(ctx, e.a, []string{ada})
	if a := activity[ada]; err != nil || a.Interactions != 1 || a.FirstAt != now.Add(-72*time.Hour).UTC().Format(time.RFC3339Nano) || a.LastAt != now.Add(-71*time.Hour).UTC().Format(time.RFC3339Nano) {
		t.Fatalf("activity: %+v %v", activity, err)
	}
}

// Logged conversations keep their people and link their records; nobody
// in another workspace can read, link or skip them.
func TestLogLinkAndSkip(t *testing.T) {
	e := setup(t, nil)
	acme, _, err := e.crm.Upsert(ctx, e.a, records.SourceUser, records.Write{Object: "companies", Set: map[string][]string{"name": {"Acme"}}})
	if err != nil {
		t.Fatal(err)
	}
	call, err := e.svc.Log(ctx, e.a, interactions.Entry{Kind: interactions.Call, People: []string{"+44 7598 490355"}, Records: []string{acme.ID}, Text: "Wants a quote by Friday."})
	if err != nil {
		t.Fatal(err)
	}
	if call.Title != "Call" || len(call.Records) != 2 || call.Text != "Wants a quote by Friday." {
		t.Fatalf("logged call: %+v", call)
	}
	person := e.contacts(t, interactions.Kept)["+447598490355"].PersonID
	if len(e.timeline(t, person)) != 1 || len(e.timeline(t, acme.ID)) != 1 {
		t.Fatal("a logged call is on its person's and record's timelines")
	}
	if latest, err := e.svc.Search(ctx, e.a, " ", 10); err != nil || len(latest) != 1 || latest[0].ID != call.ID {
		t.Fatalf("with no query, search lists the latest conversations: %+v %v", latest, err)
	}
	if latest, err := e.svc.Search(ctx, e.b, "", 10); err != nil || len(latest) != 0 {
		t.Fatalf("listed another workspace's conversations: %+v %v", latest, err)
	}
	half := interactions.Entry{Kind: interactions.Call, Title: "Half", Text: "An attempted call", People: []string{"bo@x.io", "nobody"}, Records: []string{acme.ID}}
	if _, err := e.svc.Log(ctx, e.a, half); err == nil || e.contacts(t, interactions.Pending)["bo@x.io"].Address != "" {
		t.Fatalf("a rejected log must leave nothing behind: %v", err)
	}
	if _, err := e.svc.Get(ctx, e.b, call.ID); err == nil {
		t.Error("read another workspace's call")
	}
	if _, err := e.svc.Link(ctx, e.b, call.ID, acme.ID); err == nil {
		t.Error("linked another workspace's call")
	}
	if err := e.svc.Skip(ctx, e.b, call.ID); err == nil {
		t.Error("skipped another workspace's call")
	}
	if _, err := e.svc.Log(ctx, e.b, interactions.Entry{Kind: interactions.Note, Text: "x", Records: []string{acme.ID}}); err == nil {
		t.Error("linked a log to another workspace's record")
	}
	if err := e.svc.Unlink(ctx, e.a, call.ID, acme.ID); err != nil || len(e.timeline(t, acme.ID)) != 0 {
		t.Fatalf("unlink: %v", err)
	}
	if err := e.svc.Unlink(ctx, e.a, call.ID, acme.ID); err == nil {
		t.Error("unlinked twice")
	}
	if err := e.svc.Skip(ctx, e.a, call.ID); err != nil || len(e.timeline(t, person)) != 0 {
		t.Fatalf("skip: %v", err)
	}
}

func TestLogWholeConversation(t *testing.T) {
	e := setup(t, nil)
	ada, _, err := e.crm.Upsert(ctx, e.a, records.SourceUser, records.Write{Object: "people", Set: map[string][]string{"name": {"Ada"}}})
	if err != nil {
		t.Fatal(err)
	}
	thread := interactions.Entry{Kind: interactions.Message, Channel: "LinkedIn", URL: "https://www.linkedin.com/messaging/thread/t1/", Records: []string{ada.ID}, Messages: []interactions.Said{
		{At: "2026-09-19T16:55:00Z", Sender: "Owner", Recipients: []string{"Ada"}, Direction: "sent", Text: "Hi Ada, thanks for connecting."},
		{At: "2026-09-20T15:40:00Z", Sender: "Ada", Recipients: []string{"Owner"}, Direction: "received", Text: "Connected equipment is the hard part."},
		{At: "2026-09-30", Sender: "Owner", Recipients: []string{"Ada"}, Direction: "sent", Text: "Thanks! Here is a demo."},
	}}
	logged, err := e.svc.Log(ctx, e.a, thread)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, m := range logged.Messages {
		got = append(got, m.Direction+" "+m.Sender+" "+m.At)
	}
	want := []string{"sent Owner 2026-09-19T16:55:00Z", "received Ada 2026-09-20T15:40:00Z", "sent Owner 2026-09-30"}
	if logged.Channel != "linkedin" || logged.URL != "https://www.linkedin.com/messaging/thread/t1" || !slices.Equal(got, want) || logged.LastMessage == nil || logged.LastMessage.Text != "Thanks! Here is a demo." {
		t.Fatalf("logged conversation: %q, channel %q, last %+v", got, logged.Channel, logged.LastMessage)
	}
	activity, err := e.svc.Activities(ctx, e.a, []string{ada.ID})
	if a := activity[ada.ID]; err != nil || a.FirstAt != "2026-09-19T16:55:00Z" || !strings.HasPrefix(a.LastAt, "2026-09-30") || a.Channel != "linkedin" || a.LastMessage == nil || a.LastMessage.Direction != "sent" || a.LastMessage.Text != "Thanks! Here is a demo." {
		t.Fatalf("the conversation runs from its first to its latest message: %+v %+v %v", a, a.LastMessage, err)
	}
	thread.Messages = append(thread.Messages, interactions.Said{At: "2026-10-01", Sender: "Ada", Recipients: []string{"Owner"}, Direction: "received", Text: "Looks useful."})
	again, err := e.svc.Log(ctx, e.a, thread)
	if err != nil || again.ID != logged.ID || len(again.Messages) != 4 || len(e.timeline(t, ada.ID)) != 1 {
		t.Fatalf("a repeated import updates the one conversation: %+v %v", again, err)
	}
	reply := interactions.Entry{Kind: interactions.Message, Channel: "linkedin", URL: thread.URL, At: "2026-10-02T09:00:00Z", Sender: "Owner", Recipients: []string{"Ada"}, Direction: "sent", Text: "Great, talk soon."}
	added, err := e.svc.Log(ctx, e.a, reply)
	if err != nil || added.ID != logged.ID || len(added.Messages) != 5 || added.Messages[0].Text != "Hi Ada, thanks for connecting." || added.StartedAt != "2026-09-19T16:55:00Z" || added.LastMessage.Text != reply.Text {
		t.Fatalf("logging one message to the thread's url adds it to the conversation: %+v %v", added, err)
	}
	mixed := thread
	mixed.Text = "one message too"
	if _, err := e.svc.Log(ctx, e.a, mixed); err == nil {
		t.Error("accepted messages beside one message's fields")
	}
	broken := interactions.Entry{Kind: interactions.Message, Channel: "linkedin", Records: []string{ada.ID}, Messages: []interactions.Said{
		{At: "2026-09-19", Sender: "Owner", Recipients: []string{"Ada"}, Text: "Hello"},
		{At: "2026-09-20", Sender: "Ada", Text: "No recipients"},
	}}
	if _, err := e.svc.Log(ctx, e.a, broken); err == nil || len(e.timeline(t, ada.ID)) != 1 {
		t.Fatalf("a rejected conversation must leave nothing behind: %v", err)
	}
}

type fakeClassifier struct {
	calls  int
	before func()
}

func (f *fakeClassifier) Classify(_ context.Context, _ string, candidates []interactions.Candidate) ([]interactions.Judgement, error) {
	f.calls++
	if f.before != nil {
		f.before()
	}
	verdicts := map[string]string{"keep@a.io": "keep", "skip@b.io": "skip"}
	var out []interactions.Judgement
	for _, c := range candidates {
		if v, ok := verdicts[c.Address]; ok {
			out = append(out, interactions.Judgement{Address: c.Address, Verdict: v, Reason: "test"})
		}
	}
	return out, nil
}

// The classifier judges each undecided address once; one it leaves out
// stays pending for a person.
func TestClassifier(t *testing.T) {
	f := &fakeClassifier{}
	e := setup(t, f)
	e.ingest(t, message(e.conn, "1", "t1", "keep@a.io", "owner@cas.dev"), message(e.conn, "2", "t2", "skip@b.io", "owner@cas.dev"), message(e.conn, "3", "t3", "who@c.io", "owner@cas.dev"))
	e.triage(t)
	e.triage(t)
	if got := e.contacts(t, interactions.Kept)["keep@a.io"]; got.DecidedBy != interactions.ByAgent || got.PersonID == "" {
		t.Errorf("keep: %+v", got)
	}
	if got := e.contacts(t, interactions.Skipped)["skip@b.io"]; got.DecidedBy != interactions.ByAgent {
		t.Errorf("skip: %+v", got)
	}
	if got := e.contacts(t, interactions.Pending)["who@c.io"]; got.DecidedBy != interactions.ByAgent {
		t.Errorf("undecided: %+v", got)
	}
	if f.calls != 1 {
		t.Errorf("classifier calls: %d", f.calls)
	}
}

// Mail sent from an alias learned after it arrived counts as the
// workspace's: the alias turns internal and the person written to is kept.
func TestAliasesLearnedLaterAreOwn(t *testing.T) {
	e := setup(t, nil)
	e.ingest(t, message(e.conn, "a1", "ta", "sales@ml.test", "cara@buyer.com"))
	e.triage(t)
	if got := e.contacts(t, interactions.Pending); got["sales@ml.test"].Address == "" || got["cara@buyer.com"].Address == "" {
		t.Fatalf("before the alias is known both wait: %v", got)
	}
	if err := e.store.AddAliases(ctx, e.conn.ID, []string{"owner@cas.dev", "sales@ml.test"}); err != nil {
		t.Fatal(err)
	}
	e.triage(t)
	if got := e.contacts(t, interactions.Internal)["sales@ml.test"]; got.Address == "" {
		t.Fatalf("alias not internal: %v", e.contacts(t, interactions.Pending))
	}
	if got := e.contacts(t, interactions.Kept)["cara@buyer.com"]; got.DecidedBy != interactions.ByEngagement {
		t.Fatalf("cara: %+v", got)
	}
}

func TestManualMessageAndTranscript(t *testing.T) {
	e := setupManual(t, nil)
	person, _, err := e.crm.Upsert(ctx, e.a, records.SourceUser, records.Write{Object: "people", Set: map[string][]string{"name": {"Ada"}}})
	if err != nil {
		t.Fatal(err)
	}
	entry := interactions.Entry{Kind: interactions.Message, Channel: "linkedin", Sender: "Ada", Recipients: []string{"August"}, At: "2026-09-20", Text: "A captured preview\nOn Monday Ada wrote:\n> The original wording\nSent from my iPhone", Partial: true, Direction: "received", Records: []string{person.ID}, ExternalID: "capture-1"}
	logged, err := e.svc.Log(ctx, e.a, entry)
	if err != nil {
		t.Fatal(err)
	}
	if logged.Kind != "message" || logged.Channel != "linkedin" || logged.StartedAt != "2026-09-20" || len(logged.Messages) != 1 || logged.Text != "" || len(logged.Transcript) != 0 {
		t.Fatalf("message shape: %+v", logged)
	}
	message := logged.Messages[0]
	if message.Sender != "Ada" || message.At != "2026-09-20" || message.Text != entry.Text || message.Direction != "received" || !message.Partial || !slices.Equal(message.Recipients, entry.Recipients) {
		t.Fatalf("message metadata: %+v", logged)
	}
	preview := entry
	entry.Text += "\nand the rest of the message"
	entry.Partial = false
	updated, err := e.svc.Log(ctx, e.a, entry)
	if err != nil || updated.ID != logged.ID || len(updated.Messages) != 1 || updated.Messages[0].Text != entry.Text || updated.Messages[0].Partial {
		t.Fatalf("the full message must replace its preview: %+v %v", updated, err)
	}
	again, err := e.svc.Log(ctx, e.a, preview)
	if err != nil || len(again.Messages) != 1 || again.Messages[0].Text != entry.Text || again.Messages[0].Partial {
		t.Fatalf("a preview logged again must not cut the full message: %+v %v", again, err)
	}
	entry.At = " "
	if _, err := e.svc.Log(ctx, e.a, entry); err == nil {
		t.Fatal("a missing message date silently became the import time")
	}
	entry.At = "2026-02-30"
	if _, err := e.svc.Log(ctx, e.a, entry); err == nil {
		t.Fatal("an invalid original date was accepted")
	}
	call := interactions.Entry{Kind: interactions.Call, Text: "Discussed production", Records: []string{person.ID}, ExternalID: "call-1", At: "2026-09-21T12:00:00Z", Transcript: []interactions.Speech{{Speaker: "Ada", Text: "First turn", At: "2026-09-21T12:01:00Z"}, {Speaker: "August", Text: "Second turn"}, {Speaker: "Cara", Text: "Third turn", At: "2026-09-21T12:02:00Z"}}}
	spoken, err := e.svc.Log(ctx, e.a, call)
	if err != nil || spoken.Text != call.Text || len(spoken.Transcript) != 3 || spoken.Transcript[0].Speaker != "Ada" || spoken.Transcript[1].Speaker != "August" || spoken.Transcript[2].Speaker != "Cara" {
		t.Fatalf("speaker turns: %+v %v", spoken, err)
	}
	if spoken.Transcript[0].At != call.Transcript[0].At || spoken.Transcript[1].At != "" {
		t.Fatalf("transcript timestamps were changed or invented: %+v", spoken.Transcript)
	}
	call.Text = "Revised notes"
	call.Transcript = call.Transcript[:1]
	revised, err := e.svc.Log(ctx, e.a, call)
	if err != nil || revised.ID != spoken.ID || len(revised.Transcript) != 1 || revised.Text != call.Text {
		t.Fatalf("reimport retained old notes or turns: %+v %v", revised, err)
	}
	note := interactions.Entry{Kind: interactions.Note, Text: "Research only", Records: []string{person.ID}}
	if _, err := e.svc.Log(ctx, e.a, note); err != nil {
		t.Fatal(err)
	}
	activity, err := e.svc.Activities(ctx, e.a, []string{person.ID})
	if err != nil || activity[person.ID].Interactions != 2 || activity[person.ID].FirstAt != "2026-09-20" || activity[person.ID].LastAt != "2026-09-21T12:00:00Z" {
		t.Fatalf("notes changed contact statistics: %+v %v", activity, err)
	}
	note.Transcript = []interactions.Speech{{Speaker: "Ada", Text: "Something said"}}
	if _, err := e.svc.Log(ctx, e.a, note); err == nil {
		t.Fatal("a note accepted a transcript")
	}
	call.Transcript[0].Speaker = ""
	if _, err := e.svc.Log(ctx, e.a, call); err == nil {
		t.Fatal("a transcript accepted an unattributed turn")
	}
	recent := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	untimed, err := e.svc.Log(ctx, e.a, interactions.Entry{Kind: interactions.Call, At: recent.Format(time.RFC3339Nano), Records: []string{person.ID}, Transcript: []interactions.Speech{{Speaker: "Ada", Text: "An untimed transcript"}}})
	if err != nil || len(untimed.Transcript) != 1 || untimed.Transcript[0].At != "" {
		t.Fatalf("untimed transcript: %+v %v", untimed, err)
	}
	candidates, err := e.store.FollowUpCandidates(ctx, e.a.WorkspaceID, recent.Add(-time.Minute), 10)
	i := slices.IndexFunc(candidates, func(c storage.FollowUpCandidate) bool { return c.ID == untimed.ID })
	if err != nil || i < 0 || !candidates[i].LatestAt.Equal(recent) {
		t.Fatalf("untimed transcript disappeared from follow-up discovery: %+v %v", candidates, err)
	}
}

// Chats name people by profile: a handle joins the person whose profile
// field links to it, and a stranger waits in triage. A synced meeting takes
// a transcript and notes from a recorder, but its messages and details stay
// with sync.
func TestLogProfilesAndSyncedConversations(t *testing.T) {
	e := setup(t, nil)
	caleb, _, err := e.crm.Upsert(ctx, e.a, records.SourceUser, records.Write{Object: "people", Set: map[string][]string{"name": {"Caleb"}, "x_url": {"https://x.com/OSHBuilt"}}})
	if err != nil {
		t.Fatal(err)
	}
	dm := interactions.Entry{Kind: interactions.Message, Channel: "x", URL: "https://x.com/messages/1-2", People: []string{"@OSHBuilt"}, Records: []string{caleb.ID}, At: "2026-10-08", Sender: "Caleb", Recipients: []string{"Owner"}, Direction: "received", Text: "Happy to chat."}
	logged, err := e.svc.Log(ctx, e.a, dm)
	if err != nil || len(logged.Participants) != 1 || logged.Participants[0].Address != "x.com/oshbuilt" || logged.Participants[0].PersonID != caleb.ID {
		t.Fatalf("an X handle joins the person with that profile: %+v %v", logged.Participants, err)
	}
	stranger := interactions.Entry{Kind: interactions.Message, Channel: "x", People: []string{"https://twitter.com/Stranger"}, At: "2026-10-08", Sender: "Stranger", Recipients: []string{"Owner"}, Text: "Hello"}
	if _, err := e.svc.Log(ctx, e.a, stranger); err != nil || e.contacts(t, interactions.Pending)["x.com/stranger"].Address == "" {
		t.Fatalf("a handle nobody's profile links to waits in triage: %v %v", err, e.contacts(t, interactions.Pending))
	}
	stranger.URL = "linkedin thread"
	if _, err := e.svc.Log(ctx, e.a, stranger); err == nil {
		t.Fatal("logged a conversation whose url is not a link")
	}
	stranger.URL, stranger.Channel = "", "fax"
	if _, err := e.svc.Log(ctx, e.a, stranger); err == nil {
		t.Fatal("logged a message on an unknown channel")
	}
	known, err := e.svc.Known(ctx, e.a.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	err = e.svc.IngestMeeting(ctx, known, interactions.CalendarEvent{
		ConnectionID: e.conn.ID, UserID: e.conn.UserID, ExternalID: "ev1", Title: "Line review", Start: start, End: start.Add(time.Hour), URL: "https://calendar.example/ev1",
		Attendees: []interactions.Attendee{{Email: "owner@cas.dev", Organizer: true}, {Email: "ada@customer.io", Name: "Ada"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	meeting, err := e.store.InteractionByExternalID(ctx, e.a.WorkspaceID, "", "ev1")
	if err != nil {
		t.Fatal(err)
	}
	recorded, err := e.svc.Log(ctx, e.a, interactions.Entry{Kind: interactions.Meeting, InteractionID: meeting, URL: "https://notes.example/1", Text: "Ada wants 200 a week.", Transcript: []interactions.Speech{{Speaker: "Ada", Text: "Can you do 200 a week?"}}})
	if err != nil || recorded.ID != meeting || recorded.Title != "Line review" || recorded.URL != "https://calendar.example/ev1" || len(recorded.Participants) != 2 || len(recorded.Transcript) != 1 || recorded.Text != "Ada wants 200 a week." {
		t.Fatalf("a recorder adds to the synced meeting and leaves its details: %+v %v", recorded, err)
	}
	thread := message(e.conn, "m1", "t1", "ada@customer.io", "owner@cas.dev")
	thread.URL = "https://mail.example/t1"
	e.ingest(t, thread)
	email, err := e.store.InteractionByExternalID(ctx, e.a.WorkspaceID, "email", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := e.svc.Get(ctx, e.a, email); err != nil || got.Kind != interactions.Message || got.Channel != "email" || got.URL != thread.URL {
		t.Fatalf("an email thread is a message on the email channel with its link: %+v %v", got, err)
	}
	if _, err := e.svc.Log(ctx, e.a, interactions.Entry{Kind: interactions.Message, Channel: "email", InteractionID: email, At: "2026-10-08", Sender: "Owner", Recipients: []string{"Ada"}, Text: "Logged by hand"}); err == nil {
		t.Fatal("logged a message into a synced email thread")
	}
}
