package followups_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/gluonfield/jaz-tasks/auth"
)

var ctx = context.Background()

type syncer struct{}

func (syncer) Start(context.Context, string) error          { return nil }
func (syncer) Stop(context.Context, string) error           { return nil }
func (syncer) Step(context.Context, string) (string, error) { return "", nil }

// gmail is owner@cas.dev's mailbox: it holds Jane's messages m2 and m3 in
// thread t9 and records what is sent.
type gmail struct {
	// fail answers a send with an error; drop cuts the connection instead,
	// leaving whether it went out unknown.
	fail, drop bool
	sent       []*mail.Message
	account    string
	headers    map[string]string
}

func (g *gmail) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/token":
		fmt.Fprint(w, `{"access_token":"at","token_type":"Bearer","expires_in":3600,"refresh_token":"rt"}`)
	case "/gmail/v1/users/me/profile":
		account := g.account
		if account == "" {
			account = "owner@cas.dev"
		}
		fmt.Fprintf(w, `{"emailAddress":%q,"historyId":"1"}`, account)
	case "/gmail/v1/users/me/messages/g2", "/gmail/v1/users/me/messages/g3":
		id := strings.TrimPrefix(r.URL.Path, "/gmail/v1/users/me/messages/g")
		headers := map[string]string{"From": "Jane <jane@acme.com>", "Subject": "Quote for 500 brackets", "To": "owner@cas.dev", "Cc": "bob@acme.com, owner@cas.dev", "Message-ID": "<m" + id + "@acme.com>", "References": "<m1@acme.com>"}
		for name, value := range g.headers {
			headers[name] = value
		}
		var fields []map[string]string
		for name, value := range headers {
			fields = append(fields, map[string]string{"name": name, "value": value})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "g" + id, "threadId": "t9", "internalDate": "0", "payload": map[string]any{"headers": fields}})
	case "/gmail/v1/users/me/messages":
		fmt.Fprint(w, `{"messages":[{"id":"g2","threadId":"t9"}]}`)
	case "/gmail/v1/users/me/settings/sendAs/owner@cas.dev", "/gmail/v1/users/me/settings/sendAs/mate@cas.dev":
		fmt.Fprint(w, `{"sendAsEmail":"owner@cas.dev","signature":"<div>Owner Name<br>CAS</div>"}`)
	case "/gmail/v1/users/me/messages/send":
		if g.drop {
			panic(http.ErrAbortHandler)
		}
		if g.fail {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		var body struct{ Raw, ThreadID string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		raw, _ := base64.URLEncoding.DecodeString(body.Raw)
		m, _ := mail.ReadMessage(strings.NewReader(string(raw)))
		m.Header["X-Thread"] = []string{body.ThreadID}
		g.sent = append(g.sent, m)
		fmt.Fprint(w, `{"id":"s1","threadId":"t9"}`)
	default:
		http.NotFound(w, r)
	}
}

// plainText is a sent message's plain-text body.
func plainText(t *testing.T, m *mail.Message) string {
	_, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	part, err := multipart.NewReader(m.Body, params["boundary"]).NextRawPart()
	if err != nil {
		t.Fatal(err)
	}
	text, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, part))
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

func TestReleaseSendsEmailRepliesAndApprovesOthers(t *testing.T) {
	for _, transport := range []string{"browser", "mcp"} {
		t.Run(transport, func(t *testing.T) {
			releaseSendsEmailRepliesAndApprovesOthers(t, transport)
		})
	}
}

func releaseSendsEmailRepliesAndApprovesOthers(t *testing.T, transport string) {
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
	if err != nil || mate.WorkspaceID != owner.WorkspaceID {
		t.Fatalf("teammate did not join: %+v %v", mate, err)
	}
	mateActor := auth.Actor{UserID: mate.ID, WorkspaceID: mate.WorkspaceID}
	g := &gmail{}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	conns, err := connections.NewService(store, connections.Config{
		Google:    google.OAuthConfig{ClientID: "client", ClientSecret: "secret", TokenURL: srv.URL + "/token"},
		Key:       []byte("0123456789abcdef0123456789abcdef"),
		Endpoints: google.Endpoints{Gmail: srv.URL},
	}, syncer{})
	if err != nil {
		t.Fatal(err)
	}
	mailbox, err := conns.Connect(ctx, ownerActor, "code", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	crm := records.NewService(store)
	svc := followups.NewService(crm, store, conns, store)
	thread, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: owner.WorkspaceID, ExternalID: "t9", ConnectionID: &mailbox.ID, Title: "Quote for 500 brackets", At: time.Now().Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	message := func(id string, at time.Time) {
		provider := "g" + id[1:2]
		if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: thread, Kind: "message", ExternalID: id, ConnectionID: &mailbox.ID, ProviderID: &provider, At: &at}); err != nil {
			t.Fatal(err)
		}
	}
	message("m2@acme.com", time.Now().Add(-time.Hour))
	older, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: owner.WorkspaceID, ExternalID: "t8", ConnectionID: &mailbox.ID, Title: "Intro", At: time.Now().Add(-30 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	gone := "g0"
	olderAt := time.Now().Add(-2 * time.Hour)
	if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: older, Kind: "message", ExternalID: "m0@acme.com", ConnectionID: &mailbox.ID, ProviderID: &gone, At: &olderAt}); err != nil {
		t.Fatal(err)
	}
	followUp := func(channel string) string {
		agent := ownerActor
		agent.Agent = true
		f, _, err := crm.Upsert(ctx, agent, records.SourceAgent, records.Write{Object: records.FollowUps, Set: map[string][]string{
			"name": {"Reply to Jane"}, "draft": {"Hi Jane, the revised quote is attached."}, "channel": {channel},
			"to": {"jane@acme.com"}, "cc": {"bob@acme.com"},
		}})
		if err != nil {
			t.Fatal(err)
		}
		for _, conversation := range []string{thread, older} {
			if err := store.AddLink(ctx, conversation, f.ID, "agent"); err != nil {
				t.Fatal(err)
			}
		}
		return f.ID
	}
	state := func(id string) (string, string) {
		r, err := crm.Get(ctx, ownerActor, id)
		if err != nil {
			t.Fatal(err)
		}
		pick := func(attr string) string {
			for _, f := range r.Fields {
				if f.Attribute == attr {
					return f.Values[0].Text
				}
			}
			return ""
		}
		return pick("draft_status"), pick("status")
	}

	seen := followups.Seen{Confirmed: true, Draft: "Hi Jane, the revised quote is attached.", From: "owner@cas.dev", To: []string{"jane@acme.com"}, Cc: []string{"bob@acme.com"}}
	reply := followUp("Email")
	agent := ownerActor
	agent.Agent = true
	manual, _, err := crm.Upsert(ctx, agent, records.SourceAgent, records.Write{Object: records.FollowUps, Set: map[string][]string{"name": {"Check the quote, then reply"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddLink(ctx, thread, manual.ID, "agent"); err != nil {
		t.Fatal(err)
	}
	preview, err := svc.Sender(ctx, mateActor, manual.ID)
	if err != nil || preview.From != seen.From || !slices.Equal(preview.To, seen.To) || !slices.Equal(preview.Cc, seen.Cc) {
		t.Fatalf("a follow-up without a generated draft must offer its mailbox and reply-all recipients: %+v %v", preview, err)
	}
	manualText := "Hi Jane, I will check the requirements and get back to you."
	if _, _, err := crm.Upsert(ctx, mateActor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: manual.ID, Set: map[string][]string{
		"draft": {manualText}, "channel": {"Email"}, "to": preview.To, "cc": preview.Cc,
	}}); err != nil {
		t.Fatal(err)
	}
	updated, skipped, err := crm.Upsert(ctx, agent, records.SourceAgent, records.Write{Object: records.FollowUps, RecordID: manual.ID, Set: map[string][]string{"draft": {"A later generated reply"}}})
	if err != nil || !slices.ContainsFunc(skipped, func(s records.Skip) bool { return s.Attribute == "draft" && s.Source == records.SourceUser }) || !slices.ContainsFunc(updated.Fields, func(f records.Field) bool {
		return f.Attribute == "draft" && len(f.Values) == 1 && f.Values[0].Text == manualText
	}) {
		t.Fatalf("a later generated draft must preserve the person's saved text: %+v %+v %v", updated, skipped, err)
	}
	unconfirmed := seen
	unconfirmed.Confirmed = false
	if _, err := svc.Release(ctx, agent, reply, unconfirmed); err == nil {
		t.Fatal("an unconfirmed draft was sent")
	}
	if _, err := svc.Release(ctx, mateActor, reply, followups.Seen{Confirmed: true, Draft: "An older text", To: seen.To, Cc: seen.Cc}); err == nil {
		t.Fatal("a draft was sent that differs from what the person saw")
	}
	if sender, err := svc.Sender(ctx, mateActor, reply); err != nil || sender.From != seen.From || sender.Signature != "<div>Owner Name<br>CAS</div>" || !slices.Equal(sender.To, seen.To) || !slices.Equal(sender.Cc, seen.Cc) {
		t.Fatalf("a teammate's reply must carry its mailbox, signature and reply-all recipients: %+v %v", sender, err)
	}
	if _, err := svc.Release(ctx, mateActor, reply, followups.Seen{Confirmed: true, Draft: seen.Draft, From: "mate@cas.dev", To: seen.To, Cc: seen.Cc}); err == nil || len(g.sent) != 0 {
		t.Fatalf("a draft was sent from a mailbox other than the one the person saw: %v", err)
	}
	g.fail = true
	if _, err := svc.Release(ctx, mateActor, reply, seen); err == nil {
		t.Fatal("a failed send reported success")
	}
	if draft, _ := state(reply); draft != records.DraftApproved || len(g.sent) != 0 {
		t.Fatalf("a refused send must leave the draft approved for another try: %q", draft)
	}
	g.fail, g.drop = false, true
	if _, err := svc.Release(ctx, mateActor, reply, seen); err == nil {
		t.Fatal("a send of unknown outcome reported success")
	}
	if draft, _ := state(reply); draft != records.DraftSending {
		t.Fatalf("a send that may have gone out must stay claimed: %q", draft)
	}
	if _, _, err := crm.Upsert(ctx, mateActor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: reply, Set: map[string][]string{"draft_status": {records.DraftApproved}}}); err != nil {
		t.Fatalf("a person could not release a stuck claim: %v", err)
	}
	g.drop = false
	if err := conns.SetTeammatesSend(ctx, ownerActor, mailbox.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Release(ctx, mateActor, reply, seen); err == nil || len(g.sent) != 0 {
		t.Fatalf("a teammate sent from a mailbox closed to teammates: %v", err)
	}
	if err := conns.SetTeammatesSend(ctx, ownerActor, mailbox.ID, true); err != nil {
		t.Fatal(err)
	}
	releaseThroughHTTP(t, auth.NewService(store, auth.Config{PublicURL: "http://crm.test"}), mcpapi.Services{Records: crm, Workspaces: people, Connections: conns, FollowUps: svc}, mate.ID, reply, seen, transport)
	if draft, status := state(reply); draft != records.DraftSent || status != "Done" {
		t.Fatalf("a sent reply closes its follow-up: %q %q", draft, status)
	}
	if len(g.sent) != 1 {
		t.Fatalf("sent %d messages", len(g.sent))
	}
	h := g.sent[0].Header
	got := []string{h.Get("X-Thread"), h.Get("From"), h.Get("To"), h.Get("Cc"), h.Get("Subject"), h.Get("In-Reply-To"), h.Get("References")}
	want := []string{"t9", "owner@cas.dev", "jane@acme.com", "bob@acme.com", "Re: Quote for 500 brackets", "<m2@acme.com>", "<m1@acme.com> <m2@acme.com>"}
	if !slices.Equal(got, want) {
		t.Fatalf("reply headers %q, want %q", got, want)
	}
	if text := plainText(t, g.sent[0]); text != seen.Draft+"\n\nOwner Name\nCAS" {
		t.Fatalf("a reply must carry its sender's Gmail signature below the draft: %q", text)
	}

	stale := followUp("Email")
	message("m3@acme.com", time.Now().Add(time.Minute))
	if _, err := svc.Release(ctx, ownerActor, stale, seen); err == nil || len(g.sent) != 1 {
		t.Fatalf("a draft older than the conversation's latest message was sent: %v", err)
	}

	linkedIn := followUp("LinkedIn")
	if _, err := svc.Release(ctx, mateActor, linkedIn, seen); err != nil {
		t.Fatal(err)
	}
	if draft, status := state(linkedIn); draft != records.DraftApproved || status != "Open" || len(g.sent) != 1 {
		t.Fatalf("a LinkedIn draft is approved for its sender, not sent: %q %q", draft, status)
	}
}
