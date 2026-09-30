package interactions_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
)

func TestEmailConversationView(t *testing.T) {
	e := setup(t, nil)
	addresses := []string{"owner@cas.dev", "ada@customer.io", "SALES@ml.test", "b@other.dev", "pat@cas.dev"}
	for i, address := range addresses {
		m := message(e.conn, fmt.Sprintf("m%d", i), "thread", address, "owner@cas.dev", "ada@customer.io")
		m.From.Name = "Alex"
		m.Date = start.Add(time.Duration(i) * time.Hour)
		if i > 0 {
			m.InReplyTo = "m0@mail"
		}
		e.ingest(t, m)
	}
	e.triage(t)
	ada := e.contacts(t, interactions.Kept)["ada@customer.io"].PersonID
	list := e.timeline(t, ada)
	if len(list) != 1 {
		t.Fatalf("timeline: %+v", list)
	}
	full, err := e.svc.Get(ctx, e.a, list[0].ID)
	if err != nil || len(full.Parts) != len(addresses) || full.Parts[2].Direction != "received" {
		t.Fatalf("before alias discovery: %+v %v", full, err)
	}
	if err := e.store.AddAliases(ctx, e.conn.ID, []string{"sales@ml.test"}); err != nil {
		t.Fatal(err)
	}
	due, err := e.svc.Unfetched(ctx, e.conn.ID, 50)
	if err != nil || len(due) != len(addresses) {
		t.Fatalf("message content: %+v %v", due, err)
	}
	for _, part := range due {
		if err := e.svc.SetContent(ctx, part.ID, "Body of "+part.ProviderID); err != nil {
			t.Fatal(err)
		}
	}
	full, err = e.svc.Get(ctx, e.a, list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	directions := []string{"sent", "received", "sent", "received", "received"}
	for i, part := range full.Parts {
		address := addresses[i]
		if i == 2 {
			address = "sales@ml.test"
		}
		if part.Author != "Alex" || part.AuthorAddress != address || part.Direction != directions[i] || !part.At.Equal(start.Add(time.Duration(i)*time.Hour)) {
			t.Errorf("message %d: %+v", i, part)
		}
	}
	latest := full.LastMessage
	if latest == nil || latest.AuthorAddress != "pat@cas.dev" || latest.Direction != "received" || latest.Content != "Body of m4" || !latest.At.Equal(start.Add(4*time.Hour)) || full.Preview != latest.Content {
		t.Fatalf("latest message: %+v, preview %q", latest, full.Preview)
	}
	list = e.timeline(t, ada)
	if list[0].LastMessage == nil || *list[0].LastMessage != *latest || len(list[0].Parts) != 0 || list[0].Preview != latest.Content {
		t.Fatalf("list summary differs from conversation: %+v", list[0])
	}
	if _, err := e.svc.Get(ctx, e.b, full.ID); err == nil {
		t.Fatal("another workspace read the conversation")
	}
}
