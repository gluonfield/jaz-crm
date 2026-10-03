package records_test

import (
	"slices"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

func TestActionDateAndWaitingPolicies(t *testing.T) {
	svc, actor, _ := setup(t)
	f, _ := upsert(t, svc, actor, records.SourceAgent, records.Write{Object: records.FollowUps, Set: set(
		"name", "Send documents", "waiting_on", "Us", "action_date", "2026-10-02T17:00:00+01:00",
		"action_date_basis", "Suggested", "action_date_reason", "End of the originating business week", "draft", "The documents are ready.",
	)})
	write := func(source records.Source, fields map[string][]string, remove ...string) records.Record {
		t.Helper()
		w := records.Write{Object: records.FollowUps, RecordID: f.ID, Set: fields, Remove: map[string][]string{}}
		for _, slug := range remove {
			w.Remove[slug] = nil
		}
		f, _ = upsert(t, svc, actor, source, w)
		return f
	}
	assert := func(slug string, want ...string) {
		t.Helper()
		if got := values(f, slug); !slices.Equal(got, want) {
			t.Fatalf("%s = %v, want %v", slug, got, want)
		}
	}
	assert("action_date", "2026-10-02T16:00:00Z")
	write(records.SourceAgent, set("name", "Send the promised documents"))
	assert("action_date", "2026-10-02T16:00:00Z")
	write(records.SourceAgent, set("waiting_on", "Them", "draft", "An unwanted chase"))
	assert("draft")
	assert("draft_status")
	assert("action_date", "2026-10-02T16:00:00Z")
	write(records.SourceUser, set("draft", "A human's unsent follow-up"))
	write(records.SourceAgent, set("draft", "Replace the human"))
	assert("draft", "A human's unsent follow-up")
	write(records.SourceUser, set("action_date", "2026-10-04"))
	assert("action_date_basis", "Manual")
	assert("action_date_reason")
	write(records.SourceAgent, set("action_date", "2026-10-05", "action_date_basis", "Suggested", "action_date_reason", "Push later"))
	assert("action_date", "2026-10-04")
	assert("action_date_basis", "Manual")
	assert("action_date_reason")
	write(records.SourceUser, nil, "action_date")
	write(records.SourceAgent, set("action_date", "2026-10-05", "action_date_basis", "Suggested", "action_date_reason", "Reintroduce date"))
	assert("action_date")
	assert("action_date_basis", "Manual")
	write(records.SourceUser, set("action_date", "2026-10-06"))
	write(records.SourceAgent, set("status", "Done"))
	assert("action_date")
	assert("action_date_basis")
	assert("draft", "A human's unsent follow-up")

	// A rejected lower-ranked status/waiting update must not drive policy.
	f, _ = upsert(t, svc, actor, records.SourceUser, records.Write{Object: records.FollowUps, Set: set("name", "User-owned state", "status", "Open", "waiting_on", "Us", "action_date", "2026-10-07")})
	write(records.SourceAgent, set("status", "Done", "waiting_on", "Them", "draft", "A valid reply"))
	assert("status", "Open")
	assert("waiting_on", "Us")
	assert("action_date", "2026-10-07")
	assert("draft", "A valid reply")

	write(records.SourceUser, set("draft_status", records.DraftApproved))
	write(records.SourceAgent, set("draft_status", records.DraftSending))
	write(records.SourceUser, set("waiting_on", "Them"))
	assert("draft_status", records.DraftSending)
	write(records.SourceAgent, set("draft_status", records.DraftSent))
	assert("draft_status", records.DraftSent)
	assert("draft")
}

func TestActionDateSearchUsesWorkspaceTimeAndStablePages(t *testing.T) {
	store := postgrestest.New(t)
	ws := workspaces.NewService(store, workspaces.Config{})
	u, err := ws.Provision(ctx, "dates@example.test")
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{WorkspaceID: u.WorkspaceID, UserID: u.ID}
	zone := "Europe/London"
	if _, err := ws.Update(ctx, actor, storage.WorkspaceUpdate{Timezone: &zone}); err != nil {
		t.Fatal(err)
	}
	svc := records.NewService(store)
	var ids []string
	for _, date := range []string{"", "2026-10-25", "2026-10-25T01:30:00+00:00", "2026-10-25T01:30:00+01:00", "2026-10-25T01:30:00.000001+01:00"} {
		fields := set("name", "Deadline "+date, "waiting_on", "Them")
		if date != "" {
			fields["action_date"] = []string{date}
		}
		f, _ := upsert(t, svc, actor, records.SourceUser, records.Write{Object: records.FollowUps, Set: fields})
		ids = append(ids, f.ID)
	}
	var sorted []string
	for offset := 0; offset < len(ids); offset += 2 {
		rows, _, err := svc.Search(ctx, actor, records.Search{Object: records.FollowUps, Sort: "action_date", Limit: 2, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			sorted = append(sorted, row.ID)
		}
	}
	if !slices.Equal(sorted, []string{ids[3], ids[4], ids[2], ids[1], ids[0]}) {
		t.Fatalf("timed deadlines and date-only day end must sort before undated: %v", sorted)
	}
	for _, probe := range []struct {
		at    string
		count int
	}{{"2026-10-25T00:30:00Z", 1}, {"2026-10-25T23:59:59Z", 3}, {"2026-10-26T00:00:00Z", 4}} {
		rows, _, err := svc.Search(ctx, actor, records.Search{Object: records.FollowUps, Filters: []storage.RecordFilter{{Attribute: "action_date", Operator: "on_or_before", Value: probe.at}}})
		if err != nil || len(rows) != probe.count {
			t.Fatalf("deadline cutoff %s: %d records, want %d: %v", probe.at, len(rows), probe.count, err)
		}
	}
	for _, date := range []string{time.Now().Add(-time.Hour).Format(time.RFC3339), time.Now().Add(time.Hour).Format(time.RFC3339)} {
		upsert(t, svc, actor, records.SourceUser, records.Write{Object: records.FollowUps, Set: set("name", "Clock test", "waiting_on", "Them", "action_date", date)})
	}
	today, _ := upsert(t, svc, actor, records.SourceUser, records.Write{Object: records.FollowUps, Set: set("name", "Today's deadline", "action_date", time.Now().Format(time.RFC3339))})
	sameDay, _, err := svc.Search(ctx, actor, records.Search{Object: records.FollowUps, Where: map[string]string{"name": "Today's deadline"}, Filters: []storage.RecordFilter{{Attribute: "action_date", Operator: "is", Value: "today"}}})
	if err != nil || len(sameDay) != 1 || sameDay[0].ID != today.ID {
		t.Fatalf("is today must include timed values in the workspace day: %v %v", sameDay, err)
	}
	filters, err := svc.SavedFilters(ctx, actor, records.FollowUps)
	if err != nil {
		t.Fatal(err)
	}
	chase := slices.IndexFunc(filters, func(f storage.SavedFilter) bool { return f.Name == "Chase" })
	if chase < 0 {
		t.Fatal("new workspace has no Chase preset")
	}
	rows, _, err := svc.Search(ctx, actor, records.Search{Object: records.FollowUps, Filters: filters[chase].Filters})
	if err != nil {
		t.Fatal(err)
	}
	clocks := 0
	for _, row := range rows {
		if slices.Equal(values(row, "name"), []string{"Clock test"}) {
			clocks++
		}
	}
	if clocks != 1 {
		t.Fatalf("Chase must contain only the expired timed action: %d", clocks)
	}
}
