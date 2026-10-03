package records_test

import (
	"slices"
	"sync"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/records"
)

func TestFollowUpDraftLifecycle(t *testing.T) {
	svc, a, _ := setup(t)
	agent := a
	agent.Agent = true
	jane, _ := upsert(t, svc, a, records.SourceSync, records.Write{Object: "people", Set: set("name", "Jane", "email_addresses", "jane@acme.test")})
	followUp, _ := upsert(t, svc, agent, records.SourceAgent, records.Write{Object: records.FollowUps, Set: set(
		"name", "Send the revised quote", "waiting_on", "Us", "action_date", "2026-10-02", "action_date_basis", "Stated", "action_date_reason", "By October 2", "person", jane.ID,
		"draft", "Hi Jane, the revised quote is attached.", "channel", "Email", "to", "jane@acme.test",
	)})
	status := func(r records.Record) string {
		return slices.Concat(values(r, "draft_status"), []string{""})[0]
	}
	if got := status(followUp); got != records.DraftWritten || !slices.Equal(values(followUp, "status"), []string{"Open"}) {
		t.Fatalf("a new draft starts written in an open follow-up: %q %v", got, values(followUp, "status"))
	}
	write := func(source records.Source, pairs ...string) (records.Record, error) {
		actor := a
		actor.Agent = source != records.SourceUser
		record, _, err := svc.Upsert(ctx, actor, source, records.Write{Object: records.FollowUps, RecordID: followUp.ID, Set: set(pairs...)})
		return record, err
	}
	if _, err := write(records.SourceAgent, "draft_status", records.DraftApproved); err == nil {
		t.Fatal("an agent approved a draft")
	}
	if _, err := write(records.SourceAgent, "draft_status", records.DraftSending); err == nil {
		t.Fatal("an unapproved draft was claimed for sending")
	}
	approved, err := write(records.SourceUser, "draft_status", records.DraftApproved)
	if err != nil || status(approved) != records.DraftApproved {
		t.Fatalf("a person could not approve: %v %v", status(approved), err)
	}
	edited, err := write(records.SourceAgent, "draft", "Hi Jane, the revised quote for 500 units is attached.")
	if err != nil || status(edited) != records.DraftWritten {
		t.Fatalf("editing an approved draft must withdraw approval: %q %v", status(edited), err)
	}
	if _, err := write(records.SourceUser, "draft_status", records.DraftApproved); err != nil {
		t.Fatal(err)
	}
	subjectEdit, err := write(records.SourceUser, "subject", "Revised quote")
	if err != nil || status(subjectEdit) != records.DraftWritten {
		t.Fatalf("subject editing must withdraw approval: %v %v", subjectEdit, err)
	}
	if _, err := write(records.SourceUser, "draft_status", records.DraftApproved); err != nil {
		t.Fatal(err)
	}
	if _, err := write(records.SourceAgent, "draft", "Unapproved text", "draft_status", records.DraftSending); err == nil {
		t.Fatal("an agent rewrote an approved draft and claimed it in one write")
	}
	var claims sync.WaitGroup
	won := make(chan bool, 8)
	for range 8 {
		claims.Go(func() {
			_, err := write(records.SourceAgent, "draft_status", records.DraftSending)
			won <- err == nil
		})
	}
	claims.Wait()
	close(won)
	winners := 0
	for w := range won {
		if w {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("an approved draft must be claimed exactly once, got %d claims", winners)
	}
	if _, err := write(records.SourceUser, "draft", "A late edit"); err == nil {
		t.Fatal("a draft being sent was edited")
	}
	if _, err := write(records.SourceUser, "subject", "Late subject"); err == nil {
		t.Fatal("the subject changed while sending")
	}
	if _, _, err := svc.Upsert(ctx, a, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: followUp.ID, Remove: map[string][]string{"draft_status": {}}}); err == nil {
		t.Fatal("a draft being sent lost its claim")
	}
	sent, err := write(records.SourceAgent, "draft_status", records.DraftSent)
	if err != nil || status(sent) != records.DraftSent {
		t.Fatalf("the claimer could not mark the draft sent: %q %v", status(sent), err)
	}
	if len(values(sent, "draft"))+len(values(sent, "subject")) != 0 {
		t.Fatal("sent text must leave the draft field")
	}
	if _, err := write(records.SourceAgent, "draft_status", records.DraftSent); err == nil {
		t.Fatal("a sent draft was sent again")
	}
	fresh, err := write(records.SourceAgent, "draft", "Hi Jane, we also support assembly.")
	if err != nil || status(fresh) != records.DraftWritten || !slices.Equal(values(fresh, "draft"), []string{"Hi Jane, we also support assembly."}) {
		t.Fatalf("a new reply must start a fresh draft: %+v %v", fresh, err)
	}
}

func TestFollowUpsAreReferencedAndSortedByReviewDate(t *testing.T) {
	svc, a, _ := setup(t)
	jane, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: "people", Set: set("name", "Jane")})
	var ids []string
	for _, review := range []string{"2026-10-05", "", "2026-10-02"} {
		pairs := []string{"name", "Follow up " + review, "person", jane.ID}
		if review != "" {
			pairs = append(pairs, "action_date", review)
		}
		f, _ := upsert(t, svc, a, records.SourceUser, records.Write{Object: records.FollowUps, Set: set(pairs...)})
		ids = append(ids, f.ID)
	}
	sorted, _, err := svc.Search(ctx, a, records.Search{Object: records.FollowUps, Sort: "action_date"})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, r := range sorted {
		got = append(got, r.ID)
	}
	if want := []string{ids[2], ids[0], ids[1]}; !slices.Equal(got, want) {
		t.Fatalf("follow-ups must sort earliest review first, undated last: %v, want %v", got, want)
	}
	if _, _, err := svc.Search(ctx, a, records.Search{Object: records.FollowUps, Sort: "waiting_on"}); err == nil {
		t.Fatal("sorting by a select attribute must be refused")
	}
	referenced, err := svc.Referenced(ctx, a, jane, 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := referenced.Related["follow_ups.person"]; len(got) != 3 {
		t.Fatalf("a person must list the follow-ups about them: %+v", referenced.Related)
	}
}
