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
		"name", "Send the revised quote", "waiting_on", "Us", "review_on", "2026-10-02", "person", jane.ID,
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
	if _, _, err := svc.Upsert(ctx, a, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: followUp.ID, Remove: map[string][]string{"draft_status": {}}}); err == nil {
		t.Fatal("a draft being sent lost its claim")
	}
	sent, err := write(records.SourceAgent, "draft_status", records.DraftSent)
	if err != nil || status(sent) != records.DraftSent {
		t.Fatalf("the claimer could not mark the draft sent: %q %v", status(sent), err)
	}
	if _, err := write(records.SourceAgent, "draft_status", records.DraftSent); err == nil {
		t.Fatal("a sent draft was sent again")
	}
}
