package followups_test

import (
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	signin "github.com/gluonfield/jaz-tasks/auth"
)

func TestReplyAllRecipientsThroughPreviewAndSend(t *testing.T) {
	store := postgrestest.New(t)
	people := workspaces.NewService(store, workspaces.Config{})
	owner, err := people.Provision(ctx, "owner@cas.dev")
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	if _, err := people.Invite(ctx, actor, "mate@cas.dev"); err != nil {
		t.Fatal(err)
	}
	mate, err := people.SignIn(ctx, signin.Identity{Issuer: "test", Subject: "mate", Email: "mate@cas.dev", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	g := &gmail{}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	conns, err := connections.NewService(store, connections.Config{
		Google: google.OAuthConfig{ClientID: "client", ClientSecret: "secret", TokenURL: srv.URL + "/token"},
		Key:    []byte("0123456789abcdef0123456789abcdef"), Endpoints: google.Endpoints{Gmail: srv.URL},
	}, syncer{})
	if err != nil {
		t.Fatal(err)
	}
	mailbox, err := conns.Connect(ctx, actor, "code", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddAliases(ctx, mailbox.ID, []string{"sales@cas.dev"}); err != nil {
		t.Fatal(err)
	}
	mateActor := auth.Actor{UserID: mate.ID, WorkspaceID: mate.WorkspaceID}
	g.account = "mate@cas.dev"
	if _, err := conns.Connect(ctx, mateActor, "code", "verifier"); err != nil {
		t.Fatal(err)
	}
	if err := conns.SetTeammatesSend(ctx, actor, mailbox.ID, false); err != nil {
		t.Fatal(err)
	}
	crm := records.NewService(store)
	svc := followups.NewService(crm, store, conns, store)
	at := time.Now().Add(-time.Hour)
	thread, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: actor.WorkspaceID, ExternalID: "t9", ConnectionID: &mailbox.ID, Title: "Quote", At: at})
	if err != nil {
		t.Fatal(err)
	}
	provider := "g2"
	if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: thread, Kind: "message", ExternalID: "m2", ConnectionID: &mailbox.ID, ProviderID: &provider, At: &at}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name             string
		headers          map[string]string
		savedTo, savedCc []string
		to, cc           []string
		teammate         bool
	}{
		{name: "teammate in cc", headers: map[string]string{"Cc": "bob@acme.com, Mate <MATE@cas.dev>, sales@cas.dev, owner@cas.dev, BOB@acme.com"}, to: []string{"jane@acme.com"}, cc: []string{"bob@acme.com", "mate@cas.dev"}},
		{name: "teammate in to and reply-to", headers: map[string]string{"Reply-To": "quotes@acme.com", "To": "sales@cas.dev, mate@cas.dev, quotes@acme.com", "Cc": "mate@cas.dev, bob@acme.com"}, to: []string{"quotes@acme.com", "mate@cas.dev"}, cc: []string{"bob@acme.com"}},
		{name: "saved legacy defaults", headers: map[string]string{"Cc": "bob@acme.com, mate@cas.dev, sales@cas.dev"}, savedTo: []string{"jane@acme.com"}, savedCc: []string{"bob@acme.com"}, to: []string{"jane@acme.com"}, cc: []string{"bob@acme.com", "mate@cas.dev"}},
		{name: "custom recipients", headers: map[string]string{"Cc": "bob@acme.com, mate@cas.dev"}, savedTo: []string{"jane@acme.com"}, savedCc: []string{"billing@acme.com"}, to: []string{"jane@acme.com"}, cc: []string{"billing@acme.com"}},
		{name: "custom recipient without cc", headers: map[string]string{"Cc": "bob@acme.com, mate@cas.dev"}, savedTo: []string{"jane@acme.com"}, to: []string{"jane@acme.com"}},
		{name: "custom cc without to", savedCc: []string{"billing@acme.com"}, cc: []string{"billing@acme.com"}},
		{name: "own sent message", headers: map[string]string{"From": "owner@cas.dev", "To": "jane@acme.com, mate@cas.dev", "Cc": "bob@acme.com, sales@cas.dev"}, to: []string{"jane@acme.com", "mate@cas.dev"}, cc: []string{"bob@acme.com"}},
		{name: "different sending mailbox", headers: map[string]string{"Cc": "mate@cas.dev, bob@acme.com"}, to: []string{"jane@acme.com", "owner@cas.dev"}, cc: []string{"bob@acme.com"}, teammate: true},
		{name: "saved defaults from another mailbox", headers: map[string]string{"Cc": "mate@cas.dev, bob@acme.com"}, savedTo: []string{"jane@acme.com"}, savedCc: []string{"mate@cas.dev", "bob@acme.com"}, to: []string{"jane@acme.com", "owner@cas.dev"}, cc: []string{"bob@acme.com"}, teammate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g.headers = tc.headers
			g.sent = nil
			sending := actor
			from := "owner@cas.dev"
			if tc.teammate {
				sending = mateActor
				from = "mate@cas.dev"
			}
			f, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, Set: map[string][]string{
				"name": {"Reply to quote"}, "draft": {"Thanks, I will check."}, "channel": {"Email"}, "to": tc.savedTo, "cc": tc.savedCc,
			}})
			if err != nil {
				t.Fatal(err)
			}
			if err := store.AddLink(ctx, thread, f.ID, "agent"); err != nil {
				t.Fatal(err)
			}
			preview, err := svc.Sender(ctx, sending, f.ID)
			if err != nil || preview.From != from || !slices.Equal(preview.To, tc.to) || !slices.Equal(preview.Cc, tc.cc) {
				t.Fatalf("preview: %+v, error: %v; want from %s, to %v, cc %v", preview, err, from, tc.to, tc.cc)
			}
			seen := followups.Seen{Confirmed: true, Draft: "Thanks, I will check.", From: preview.From, To: preview.To, Cc: []string{"unreviewed@acme.com"}}
			if _, err := svc.Release(ctx, sending, f.ID, seen); err == nil || len(g.sent) != 0 {
				t.Fatalf("sent recipients that do not match the preview: %v", err)
			}
			seen.Cc = preview.Cc
			sent, err := svc.Release(ctx, sending, f.ID, seen)
			if err != nil || len(g.sent) != 1 {
				t.Fatalf("send: %v; messages: %d", err, len(g.sent))
			}
			h := g.sent[0].Header
			sender, _ := h.AddressList("From")
			if len(sender) != 1 || sender[0].Address != from || h.Get("To") != strings.Join(tc.to, ", ") || h.Get("Cc") != strings.Join(tc.cc, ", ") {
				t.Fatalf("sent recipients differ from the preview: %v", h)
			}
			for attribute, want := range map[string][]string{"to": tc.to, "cc": tc.cc, "draft_status": {records.DraftSent}} {
				var got []string
				for _, field := range sent.Fields {
					if field.Attribute == attribute {
						for _, value := range field.Values {
							got = append(got, value.Text)
						}
					}
				}
				if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
					t.Fatalf("saved %s = %v, want %v", attribute, got, want)
				}
			}
		})
	}
}
