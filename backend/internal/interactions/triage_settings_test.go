package interactions_test

import (
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func TestManualTriageAndIndependentRules(t *testing.T) {
	f := &fakeClassifier{}
	e := setupManual(t, f)
	e.ingest(t, message(e.conn, "sent", "sent", "owner@cas.dev", "written@buyer.io"))
	existing, _, err := e.crm.Upsert(ctx, e.a, records.SourceUser, records.Write{Object: "people", Set: map[string][]string{"email_addresses": {"existing@buyer.io"}}})
	if err != nil {
		t.Fatal(err)
	}
	e.ingest(t, message(e.conn, "existing", "existing", "existing@buyer.io", "owner@cas.dev"))
	known, err := e.svc.Known(ctx, e.a.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.IngestMeeting(ctx, known, interactions.CalendarEvent{
		ConnectionID: e.conn.ID, UserID: e.conn.UserID, ExternalID: "meet", Start: start, End: start.Add(time.Hour),
		Attendees: []interactions.Attendee{{Email: "owner@cas.dev", Organizer: true}, {Email: "met@buyer.io"}},
	}); err != nil {
		t.Fatal(err)
	}

	if err := e.svc.IngestMeeting(ctx, known, interactions.CalendarEvent{
		ConnectionID: e.conn.ID, UserID: e.conn.UserID, ExternalID: "future", Start: time.Now().Add(24 * time.Hour), End: time.Now().Add(25 * time.Hour),
		Attendees: []interactions.Attendee{{Email: "owner@cas.dev", Organizer: true}, {Email: "future@buyer.io"}},
	}); err != nil {
		t.Fatal(err)
	}
	e.triage(t)
	if kept := e.contacts(t, interactions.Kept); len(kept) != 0 || f.calls != 0 {
		t.Fatalf("manual default approved contacts or ran AI: %v, calls %d", kept, f.calls)
	}
	if pending := e.contacts(t, interactions.Pending); len(pending) != 4 {
		t.Fatalf("manual queue: %v", pending)
	}
	if err := e.store.UpdateTriageSettings(ctx, e.a.WorkspaceID, storage.TriageSettings{AutoKeepEmail: true}); err != nil {
		t.Fatal(err)
	}
	e.triage(t)
	if kept := e.contacts(t, interactions.Kept); len(kept) != 1 || kept["written@buyer.io"].PersonID == "" {
		t.Fatalf("email rule: %v", kept)
	}
	if err := e.store.UpdateTriageSettings(ctx, e.a.WorkspaceID, storage.TriageSettings{AutoKeepMeetings: true}); err != nil {
		t.Fatal(err)
	}
	e.triage(t)
	if kept := e.contacts(t, interactions.Kept); len(kept) != 2 || kept["met@buyer.io"].PersonID == "" {
		t.Fatalf("meeting rule: %v", kept)
	}
	if err := e.store.UpdateTriageSettings(ctx, e.a.WorkspaceID, storage.TriageSettings{AutoKeepRecords: true}); err != nil {
		t.Fatal(err)
	}
	e.triage(t)
	if got := e.contacts(t, interactions.Kept)["existing@buyer.io"]; got.PersonID != existing.ID {
		t.Fatalf("existing-record rule: %+v", got)
	}
}
