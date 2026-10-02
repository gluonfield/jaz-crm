package interactions_test

import (
	"fmt"
	"reflect"
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
	if err != nil || len(full.Messages) != len(addresses) || full.Messages[2].Direction != "received" {
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
		if err := e.svc.SetContent(ctx, part.ID, "Body of "+part.ProviderID, "<p>Body of <b>"+part.ProviderID+"</b></p>"); err != nil {
			t.Fatal(err)
		}
	}
	full, err = e.svc.Get(ctx, e.a, list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	directions := []string{"sent", "received", "sent", "received", "received"}
	for i, part := range full.Messages {
		address := addresses[i]
		if i == 2 {
			address = "sales@ml.test"
		}
		if part.Sender != "Alex" || part.SenderAddress != address || part.Direction != directions[i] || part.At != start.Add(time.Duration(i)*time.Hour).Format(time.RFC3339Nano) || part.HTML != fmt.Sprintf("<p>Body of <b>m%d</b></p>", i) {
			t.Errorf("message %d: %+v", i, part)
		}
	}
	latest := full.LastMessage
	if latest == nil || latest.SenderAddress != "pat@cas.dev" || latest.Direction != "received" || latest.Text != "Body of m4" || latest.HTML != "" || latest.At != start.Add(4*time.Hour).Format(time.RFC3339Nano) || full.Preview != latest.Text {
		t.Fatalf("latest message: %+v, preview %q", latest, full.Preview)
	}
	list = e.timeline(t, ada)
	if list[0].LastMessage == nil || !reflect.DeepEqual(list[0].LastMessage, latest) || len(list[0].Messages) != 0 || list[0].Preview != latest.Text {
		t.Fatalf("list summary differs from conversation: %+v", list[0])
	}
	if _, err := e.svc.Get(ctx, e.b, full.ID); err == nil {
		t.Fatal("another workspace read the conversation")
	}
	source, err := e.svc.Source(ctx, e.a, full.ID)
	if err != nil || len(source.Messages) != len(addresses) {
		t.Fatalf("source: %+v %v", source, err)
	}
	for _, message := range source.Messages {
		if message.HTML != "" || message.Text == "" {
			t.Fatalf("drafting source must retain text without duplicating HTML: %+v", message)
		}
	}
}
