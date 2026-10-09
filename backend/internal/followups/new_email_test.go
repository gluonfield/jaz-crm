package followups_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"golang.org/x/oauth2"
)

func TestNewEmailAfterCall(t *testing.T) {
	for _, transport := range []string{"browser", "mcp", "cancelled after acknowledgement", "concurrent sends", "edited during preparation"} {
		t.Run(transport, func(t *testing.T) {
			store := postgrestest.New(t)
			people := workspaces.NewService(store, workspaces.Config{})
			owner, err := people.Provision(ctx, "owner@cas.dev")
			if err != nil {
				t.Fatal(err)
			}
			actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
			g := &gmail{}
			server := httptest.NewServer(g)
			t.Cleanup(server.Close)
			conns, err := connections.NewService(store, connections.Config{
				Google: google.OAuthConfig{ClientID: "client", ClientSecret: "secret", TokenURL: server.URL + "/token"},
				Key:    []byte("0123456789abcdef0123456789abcdef"), Endpoints: google.Endpoints{Gmail: server.URL},
			}, syncer{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := conns.Connect(ctx, actor, "code", "verifier"); err != nil {
				t.Fatal(err)
			}
			crm := records.NewService(store)
			svc := followups.NewService(crm, store, conns, store)
			person, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "people", Set: map[string][]string{"name": {"Jane"}, "email_addresses": {"jane@acme.com"}}})
			if err != nil {
				t.Fatal(err)
			}
			convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
			if _, err := convs.Log(ctx, actor, interactions.Entry{Kind: "call", Text: "Jane asked us to email her the demo link, https://example.com/demo.", Records: []string{person.ID}}); err != nil {
				t.Fatal(err)
			}
			body, subject := "Hi Jane, here is the demo: https://example.com/demo.", "Your quotation demo"
			brain := &planner{plans: []followups.Plan{{FollowUps: []followups.Change{{Action: "Send Jane the demo", WaitingOn: "Us", Status: "Open", Person: person.ID, Channel: "Email", Subject: subject, Reply: body}}}}}
			agent := followups.NewAgent(followups.AgentParams{Service: svc, Workspaces: store, Interactions: convs, Logger: log.New(io.Discard), Planner: brain})
			if count, err := agent.Run(ctx, actor.WorkspaceID); err != nil || count != 1 {
				t.Fatalf("draft after a call: %d %v", count, err)
			}
			found, _, err := crm.Search(ctx, actor, records.Search{Object: "follow_ups"})
			if err != nil || len(found) != 1 {
				t.Fatalf("new email draft: %v %v", found, err)
			}
			f := found[0]
			preview, err := svc.Sender(ctx, actor, f.ID)
			if err != nil || preview.Reply || preview.From != "owner@cas.dev" || preview.Subject != subject || !slices.Equal(preview.To, []string{"jane@acme.com"}) {
				t.Fatalf("new email preview: %+v %v", preview, err)
			}
			// An existing body without a subject stays editable, but cannot be sent.
			if _, err := svc.SaveDraft(ctx, actor, f.ID, body, "", "Email", preview.To, nil); err != nil {
				t.Fatal(err)
			}
			seen := followups.Seen{Confirmed: true, Draft: body, Subject: "", From: preview.From, To: preview.To}
			if _, err := svc.Release(ctx, actor, f.ID, seen); err == nil || len(g.sent) != 0 {
				t.Fatalf("sent without a subject: %v", err)
			}
			seen.Subject = "A reviewed demo subject"
			if _, err := svc.SaveDraft(ctx, actor, f.ID, body, seen.Subject, "Email", seen.To, nil); err != nil {
				t.Fatal(err)
			}
			switch transport {
			case "edited during preparation":
				g.onIdentity = func() {
					if _, err := svc.SaveDraft(t.Context(), actor, f.ID, body, seen.Subject, "Email", []string{"changed@acme.com"}, nil); err != nil {
						t.Error(err)
					}
				}
				if _, err := svc.Release(t.Context(), actor, f.ID, seen); err == nil || g.attempts != 0 {
					t.Fatalf("a concurrent recipient edit must prevent sending: %v", err)
				}
				return
			case "concurrent sends":
				var preparing atomic.Int32
				secondReady, sendStarted := make(chan struct{}), make(chan struct{})
				g.onIdentity = func() {
					if preparing.Add(1) == 1 {
						<-secondReady
					} else {
						close(secondReady)
						<-sendStarted
					}
				}
				gate := make(chan struct{})
				var sending sync.Once
				g.onSend = func() {
					sending.Do(func() { close(sendStarted) })
					<-gate
				}
				results := make(chan error, 2)
				for range 2 {
					go func() {
						_, err := svc.Release(t.Context(), actor, f.ID, seen)
						results <- err
					}()
				}
				remaining := 2
				select {
				case err := <-results:
					remaining--
					if err == nil {
						t.Error("second sender must lose the claim")
					}
				case <-time.After(5 * time.Second):
					t.Error("both senders reached Gmail")
				}
				close(gate)
				for range remaining {
					if err := <-results; err != nil {
						t.Error(err)
					}
				}
			case "cancelled after acknowledgement":
				request, cancel := context.WithCancel(t.Context())
				defer cancel()
				request = context.WithValue(request, oauth2.HTTPClient, &http.Client{Transport: cancelAfterSend{cancel}})
				if _, err := svc.Release(request, actor, f.ID, seen); err != nil || request.Err() == nil {
					t.Fatalf("acknowledged send must finish after caller cancellation: %v, context: %v", err, request.Err())
				}
				stored, err := crm.Get(t.Context(), actor, f.ID)
				status := slices.IndexFunc(stored.Fields, func(f records.Field) bool { return f.Attribute == "draft_status" })
				if err != nil || status < 0 || len(stored.Fields[status].Values) != 1 || stored.Fields[status].Values[0].Text != records.DraftSent || slices.ContainsFunc(stored.Fields, func(f records.Field) bool { return f.Attribute == "draft" }) {
					t.Fatalf("acknowledged send was not persisted: %+v %v", stored, err)
				}
			default:
				releaseThroughHTTP(t, auth.NewService(store, auth.Config{PublicURL: "http://crm.test"}), mcpapi.Services{Records: crm, Workspaces: people, Connections: conns, FollowUps: svc}, owner.ID, f.ID, seen, transport)
			}
			if len(g.sent) != 1 {
				t.Fatalf("sent %d messages", len(g.sent))
			}
			h := g.sent[0].Header
			if h.Get("Subject") != seen.Subject || h.Get("To") != "jane@acme.com" || h.Get("Reply-To") != "sales@cas.dev" || h.Get("X-Thread") != "" || h.Get("In-Reply-To") != "" || h.Get("References") != "" {
				t.Fatalf("new email headers: %v", h)
			}
			if got := plainText(t, g.sent[0]); got != body+"\n\n-- \nOwner Name\nCAS" {
				t.Fatalf("new email body/signature: %q", got)
			}
		})
	}
}

type cancelAfterSend struct{ cancel context.CancelFunc }

func (c cancelAfterSend) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := http.DefaultTransport.RoundTrip(r)
	if err == nil && r.URL.Path == "/gmail/v1/users/me/messages/send" {
		response.Body = cancelWhenClosed{response.Body, c.cancel}
	}
	return response, err
}

type cancelWhenClosed struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c cancelWhenClosed) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}
