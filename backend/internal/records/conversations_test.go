package records_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

func TestConversationQueue(t *testing.T) {
	store := postgrestest.New(t)
	ws := workspaces.NewService(store, workspaces.Config{})
	var actors []auth.Actor
	for _, email := range []string{"queue@one.test", "queue@two.test"} {
		u, err := ws.Provision(ctx, email)
		if err != nil {
			t.Fatal(err)
		}
		actors = append(actors, auth.Actor{WorkspaceID: u.WorkspaceID, UserID: u.ID})
	}
	a, b := actors[0], actors[1]
	zone := "Europe/London"
	if _, err := ws.Update(ctx, a, storage.WorkspaceUpdate{Timezone: &zone}); err != nil {
		t.Fatal(err)
	}
	svc := records.NewService(store)
	person, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", Set: set("name", "Same Person")})
	conversation := func(actor auth.Actor, kind, name string, at time.Time, skipped bool) string {
		t.Helper()
		i, err := store.UpsertInteraction(ctx, storage.NewInteraction{WorkspaceID: actor.WorkspaceID, Kind: kind, Source: "manual", ExternalID: name, Title: name, StartedAt: at, Skipped: skipped})
		if err != nil {
			t.Fatal(err)
		}
		return i.ID
	}
	link := func(id, conversationID string) {
		t.Helper()
		if err := store.AddLink(ctx, conversationID, id, "user"); err != nil {
			t.Fatal(err)
		}
	}
	add := func(actor auth.Actor, conversationID, name, status, waiting, date string) records.Record {
		t.Helper()
		fields := set("name", name, "status", status)
		if actor.WorkspaceID == a.WorkspaceID {
			fields["person"] = []string{person.ID}
		}
		if waiting != "" {
			fields["waiting_on"] = []string{waiting}
		}
		if date != "" {
			fields["action_date"] = []string{date}
		}
		r, _ := upsert(t, svc, actor, records.SourceUser, records.Write{Object: records.FollowUps, Set: fields})
		if conversationID != "" {
			link(r.ID, conversationID)
		}
		return r
	}
	search := func(actor auth.Actor, q records.Search, total int, want ...string) []records.Record {
		t.Helper()
		q.Object = records.FollowUps
		rows, gotTotal, err := svc.Search(ctx, actor, q)
		var ids []string
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		if err != nil || gotTotal != total || !slices.Equal(ids, want) {
			t.Fatalf("query %+v: ids %v, total %d, error %v; want %v, total %d", q, ids, gotTotal, err, want, total)
		}
		return rows
	}
	old := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	thread := conversation(a, "email", "RFQ", old, false)
	var history []string
	for i := range 5 {
		r := add(a, thread, fmt.Sprintf("Historical reply %d statukai", i), "Done", "Us", "")
		history = append(history, r.ID)
	}
	q := records.Search{GroupByConversation: true, Query: "statukai"}
	rows := search(a, q, 1, history[4])
	if rows[0].ConversationID != thread {
		t.Fatalf("group must identify its exact conversation: %+v", rows[0])
	}
	search(a, records.Search{ConversationID: thread, Sort: "name"}, 5, history...)
	add(a, thread, "Dismissed stale action", "Dismissed", "Us", "")
	search(a, q, 1, history[4])
	open := add(a, thread, "Answer new question", "Open", "Us", "2026-10-25T01:30:00+01:00")
	search(a, q, 1, open.ID)
	upsert(t, svc, a, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: open.ID, Remove: map[string][]string{"status": nil, "waiting_on": nil}})
	search(a, q, 1, open.ID)
	dateOnly := add(a, thread, "Send documents", "Open", "Us", "2026-10-25")
	laterClock := add(a, thread, "Make call", "Open", "Us", "2026-10-25T01:30:00+00:00")
	add(a, thread, "Later undated request", "Open", "Us", "")
	them := add(a, thread, "Wait for drawings", "Open", "Them", "2026-10-01")
	search(a, q, 1, open.ID)
	q.Filters = []records.Filter{{Attribute: "waiting_on", Operator: "is", Value: "Them"}, {Attribute: "status", Operator: "is", Value: "Open"}}
	search(a, q, 1, them.ID)
	q.Filters = []records.Filter{{Attribute: "status", Operator: "is", Value: "Done"}}
	search(a, q, 1, history[4])
	q.Filters = nil
	upsert(t, svc, a, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: open.ID, Set: set("status", "Done")})
	search(a, q, 1, laterClock.ID)
	upsert(t, svc, a, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: laterClock.ID, Set: set("status", "Done")})
	search(a, q, 1, dateOnly.ID)

	second := conversation(a, "message", "Separate conversation", old.Add(time.Hour), false)
	separate := add(a, second, "Other statukai topic", "Open", "Us", "")
	standalone := add(a, "", "Standalone statukai action", "Open", "Us", "")
	q.Sort = "name"
	search(a, q, 3, separate.ID, dateOnly.ID, standalone.ID)
	q.Limit = 1
	for offset, id := range []string{separate.ID, dateOnly.ID, standalone.ID} {
		q.Offset = offset
		search(a, q, 3, id)
	}
	q.Offset = 3
	search(a, q, 3)
	q.Offset, q.Limit = 0, 20
	q.Query = "no-match"
	search(a, q, 0)
	q.Query = "statukai"
	private := conversation(b, "email", "Private", old, false)
	privateAction := add(b, private, "Private statukai", "Open", "Us", "")
	link(privateAction.ID, thread)
	link(standalone.ID, private)
	search(a, q, 3, separate.ID, dateOnly.ID, standalone.ID)
	search(b, q, 1, privateAction.ID)
	search(b, records.Search{ConversationID: thread}, 0)
	search(a, records.Search{ConversationID: private}, 0)
	if _, _, err := svc.Search(ctx, a, records.Search{Object: "people", GroupByConversation: true}); err == nil {
		t.Fatal("conversation grouping accepted for another object")
	}
	if _, _, err := svc.Search(ctx, a, records.Search{Object: records.FollowUps, ConversationID: "invalid"}); err == nil {
		t.Fatal("invalid interaction ID accepted")
	}

	newer := conversation(a, "email", "Newer draft-only thread", old.Add(2*time.Hour), false)
	link(separate.ID, newer)
	newerAction := add(a, newer, "Newest statukai action", "Open", "Us", "")
	skipped := conversation(a, "email", "Skipped", old.Add(3*time.Hour), true)
	link(newerAction.ID, skipped)
	meeting := conversation(a, "meeting", "Later meeting", old.Add(4*time.Hour), false)
	link(newerAction.ID, meeting)
	q.Query = "Other statukai"
	rows = search(a, q, 1, separate.ID)
	if rows[0].ConversationID != "" {
		t.Fatalf("an action linked to multiple conversations must remain standalone: %+v", rows[0])
	}
	search(a, records.Search{Query: "Other statukai"}, 1, separate.ID)
	search(a, records.Search{ConversationID: second}, 0)
	rows = search(a, records.Search{ConversationID: newer, Sort: "name"}, 1, newerAction.ID)
	if rows[0].ConversationID != newer {
		t.Fatalf("a sole eligible conversation must ignore skipped threads and meetings: %+v", rows[0])
	}
}
