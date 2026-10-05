package followups_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"slices"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	signin "github.com/gluonfield/jaz-tasks/auth"
)

func TestNewEmailKeepsAssignedSender(t *testing.T) {
	for _, scenario := range []string{"browser", "mcp", "sharing disabled", "disconnected", "owner changed during preparation", "alias"} {
		t.Run(scenario, func(t *testing.T) {
			store := postgrestest.New(t)
			people := workspaces.NewService(store, workspaces.Config{})
			owner, err := people.Provision(ctx, "owner@cas.dev")
			if err != nil {
				t.Fatal(err)
			}
			ownerActor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
			if _, err := people.Invite(ctx, ownerActor, "mate@cas.dev"); err != nil {
				t.Fatal(err)
			}
			mate, err := people.SignIn(ctx, signin.Identity{Issuer: "test", Subject: "mate", Email: "mate@cas.dev", EmailVerified: true})
			if err != nil {
				t.Fatal(err)
			}
			mateActor := auth.Actor{UserID: mate.ID, WorkspaceID: mate.WorkspaceID}
			g := &gmail{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/gmail/v1/users/me/settings/sendAs/mate@cas.dev" || r.URL.Path == "/gmail/v1/users/me/settings/sendAs/mate@university.dev" {
					if g.onIdentity != nil {
						g.onIdentity()
					}
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, `{"sendAsEmail":%q,"displayName":"Mate Name","signature":"<div>Mate Name<br>CAS</div>"}`, g.account)
					return
				}
				g.ServeHTTP(w, r)
			}))
			t.Cleanup(server.Close)
			conns, err := connections.NewService(store, connections.Config{
				Google: google.OAuthConfig{ClientID: "client", ClientSecret: "secret", TokenURL: server.URL + "/token"},
				Key:    []byte("0123456789abcdef0123456789abcdef"), Endpoints: google.Endpoints{Gmail: server.URL},
			}, syncer{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := conns.Connect(ctx, ownerActor, "code", "verifier"); err != nil {
				t.Fatal(err)
			}
			g.account = "mate@cas.dev"
			if scenario == "alias" {
				g.account = "mate@university.dev"
			}
			mailbox, err := conns.Connect(ctx, mateActor, "code", "verifier")
			if err != nil {
				t.Fatal(err)
			}
			assigned := mate.Email
			if scenario == "alias" {
				if err := conns.AddAliases(ctx, mailbox.ID, []string{assigned}); err != nil {
					t.Fatal(err)
				}
			}
			crm := records.NewService(store)
			svc := followups.NewService(crm, store, conns, store)
			body, subject := "Hi Jane, I'm Mate. Here is the demo.", "Your quotation demo"
			creator := mateActor
			creator.Agent = true
			f, _, err := crm.Upsert(ctx, creator, records.SourceAgent, records.Write{Object: records.FollowUps, Set: map[string][]string{
				"name": {"Email Jane"}, "owner": {assigned}, "channel": {"Email"}, "draft": {body}, "subject": {subject}, "to": {"jane@acme.com"},
			}})
			if err != nil {
				t.Fatal(err)
			}
			for _, viewer := range []auth.Actor{ownerActor, mateActor} {
				preview, err := svc.Sender(ctx, viewer, f.ID)
				if err != nil || preview.Reply || preview.From != mailbox.Account || !strings.Contains(preview.Signature, "Mate Name") || !slices.Equal(preview.To, []string{"jane@acme.com"}) {
					t.Fatalf("assigned sender changed with the viewer: %+v %v", preview, err)
				}
			}
			seen := followups.Seen{Confirmed: true, Draft: body, Subject: subject, From: mailbox.Account, To: []string{"jane@acme.com"}}
			wrong := seen
			wrong.From = "owner@cas.dev"
			if _, err := svc.Release(ctx, ownerActor, f.ID, wrong); err == nil || g.attempts != 0 {
				t.Fatalf("sent using the viewer's previously shown mailbox: %v", err)
			}
			switch scenario {
			case "sharing disabled":
				if err := conns.SetTeammatesSend(ctx, mateActor, mailbox.ID, false); err != nil {
					t.Fatal(err)
				}
				if preview, err := svc.Sender(ctx, ownerActor, f.ID); err == nil || preview.From != "" {
					t.Fatalf("a restricted mailbox fell back to the viewer: %+v %v", preview, err)
				}
				if preview, err := svc.Sender(ctx, mateActor, f.ID); err != nil || preview.From != "mate@cas.dev" {
					t.Fatalf("the owner lost access to their mailbox: %+v %v", preview, err)
				}
			case "disconnected":
				if err := conns.Disconnect(ctx, mateActor, mailbox.ID); err != nil {
					t.Fatal(err)
				}
				if preview, err := svc.Sender(ctx, ownerActor, f.ID); err == nil || preview.From != "" {
					t.Fatalf("a disconnected mailbox fell back to the viewer: %+v %v", preview, err)
				}
			case "owner changed during preparation":
				g.onIdentity = func() {
					if _, _, err := crm.Upsert(ctx, ownerActor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: f.ID, Set: map[string][]string{"owner": {"owner@cas.dev"}}}); err != nil {
						t.Error(err)
					}
				}
			default:
				if scenario == "alias" {
					if _, err := svc.Release(ctx, ownerActor, f.ID, seen); err != nil {
						t.Fatal(err)
					}
				} else {
					releaseThroughHTTP(t, auth.NewService(store, auth.Config{PublicURL: "http://crm.test"}), mcpapi.Services{Records: crm, Workspaces: people, Connections: conns, FollowUps: svc}, owner.ID, f.ID, seen, scenario)
				}
				if len(g.sent) != 1 {
					t.Fatalf("sent %d messages", len(g.sent))
				}
				from, err := mail.ParseAddress(g.sent[0].Header.Get("From"))
				if err != nil || from.Address != mailbox.Account || from.Name != "Mate Name" {
					t.Fatalf("sent with another person's mailbox: %q %v", g.sent[0].Header.Get("From"), err)
				}
				if got := plainText(t, g.sent[0]); got != body+"\n\n-- \nMate Name\nCAS" {
					t.Fatalf("sent with another person's signature: %q", got)
				}
				return
			}
			if _, err := svc.Release(ctx, ownerActor, f.ID, seen); err == nil || g.attempts != 0 {
				t.Fatalf("sent despite a changed or unavailable assigned sender: %v", err)
			}
		})
	}
}
