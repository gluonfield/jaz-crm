package followups_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/textproto"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

func TestImportedGmailDraftLifecycle(t *testing.T) {
	for _, outcome := range []string{"CRM send", "Gmail send", "deleted", "stale send", "blocked save", "Bcc only", "delayed sent", "sync during send"} {
		t.Run(outcome, func(t *testing.T) {
			store := postgrestest.New(t)
			owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "owner@cas.dev")
			if err != nil {
				t.Fatal(err)
			}
			actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
			provider := &draftMailbox{t: t, raw: draftMIME("Prepared in Gmail", "A prepared email"), revision: 1, present: true, at: time.Now().Add(-time.Minute)}
			if outcome == "Bcc only" {
				provider.raw = strings.Replace(provider.raw, "To: jane@acme.com\r\n", "", 1)
			}
			server := httptest.NewServer(provider)
			t.Cleanup(server.Close)
			conns, err := connections.NewService(store, connections.Config{Google: google.OAuthConfig{ClientID: "client", ClientSecret: "secret", TokenURL: server.URL + "/token"}, Key: []byte("0123456789abcdef0123456789abcdef"), Endpoints: google.Endpoints{Gmail: server.URL}}, syncer{})
			if err != nil {
				t.Fatal(err)
			}
			connection, err := conns.Connect(ctx, actor, "code", "verifier")
			if err != nil {
				t.Fatal(err)
			}
			mailbox, err := conns.Google(ctx, connection)
			if err != nil {
				t.Fatal(err)
			}
			crm := records.NewService(store)
			svc := followups.NewService(crm, store, conns, store)
			convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
			known, err := convs.Known(ctx, actor.WorkspaceID)
			if err != nil {
				t.Fatal(err)
			}
			if err := convs.IngestEmail(ctx, known, interactions.EmailMessage{ConnectionID: connection.ID, UserID: connection.UserID, ProviderID: "draft-message-1", ThreadID: "thread", From: interactions.Address{Email: connection.Account}, Date: provider.at, Labels: []string{"DRAFT"}}); err != nil {
				t.Fatal(err)
			}
			if _, err := store.InteractionByExternalID(ctx, actor.WorkspaceID, "gmail", "thread"); !errors.Is(err, storage.ErrNotFound) {
				t.Fatalf("new Gmail draft was imported as delivered conversation: %v", err)
			}
			thread, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: actor.WorkspaceID, ExternalID: "thread", ConnectionID: &connection.ID, Title: "A prepared email", At: provider.at})
			if err != nil {
				t.Fatal(err)
			}
			providerID, original := "draft-message-1", "Prepared in Gmail"
			if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: thread, Kind: "message", ExternalID: "draft-rfc-1", ConnectionID: &connection.ID, ProviderID: &providerID, At: &provider.at, Content: &original}); err != nil {
				t.Fatal(err)
			}
			legacy, _, err := crm.Upsert(ctx, actor, records.SourceAgent, records.Write{Object: records.FollowUps, Set: map[string][]string{"name": {"Wait for response"}, "status": {"Done"}, "waiting_on": {"Them"}}})
			if err != nil {
				t.Fatal(err)
			}
			if err := store.AddLink(ctx, thread, legacy.ID, interactions.ByAgent); err != nil {
				t.Fatal(err)
			}
			if err := svc.SyncGmailDrafts(ctx, connection, mailbox); err != nil {
				t.Fatal(err)
			}
			all, _, err := crm.Search(ctx, actor, records.Search{Object: records.FollowUps})
			if err != nil || len(all) != 1 || all[0].ID != legacy.ID || field(all[0], "waiting_on") != "Us" || field(all[0], "draft") != original || field(all[0], "draft_status") != records.DraftWritten {
				t.Fatalf("draft migration duplicated or lost the original follow-up: %+v %v", all, err)
			}
			parts, err := store.Parts(ctx, []string{thread})
			if err != nil || len(parts) != 1 || parts[0].Kind != "draft" || parts[0].Content == nil || *parts[0].Content != original {
				t.Fatalf("legacy content must remain stored as an unsent draft: %+v %v", parts, err)
			}
			view, err := convs.Get(ctx, actor, thread)
			if err != nil || len(view.Messages) != 0 || len(view.Drafts) != 1 || view.Drafts[0].FollowUpID != legacy.ID {
				t.Fatalf("original interaction URL must show an explicit draft: %+v %v", view, err)
			}
			source, err := convs.Source(ctx, actor, thread)
			if err != nil || len(source.Messages) != 0 || len(source.Drafts) != 0 {
				t.Fatalf("unsent draft leaked into delivered agent evidence: %+v %v", source, err)
			}
			activity, err := store.RecordActivity(ctx, actor.WorkspaceID, []string{legacy.ID})
			if err != nil || len(activity) != 0 {
				t.Fatalf("draft counted as completed contact: %+v %v", activity, err)
			}
			manualDate := time.Now().AddDate(0, 0, 2).Format(time.DateOnly)
			if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: legacy.ID, Set: map[string][]string{"action_date": {manualDate}}}); err != nil {
				t.Fatal(err)
			}
			provider.raw, provider.revision = draftMIME("Edited in Gmail", "Updated subject"), 2
			if outcome == "Bcc only" {
				provider.raw = strings.Replace(provider.raw, "To: jane@acme.com\r\n", "", 1)
			}
			if err := svc.SyncGmailDrafts(ctx, connection, mailbox); err != nil {
				t.Fatal(err)
			}
			record, _ := crm.Get(ctx, actor, legacy.ID)
			if field(record, "draft") != "Edited in Gmail" || field(record, "subject") != "Updated subject" || field(record, "action_date") != manualDate {
				t.Fatalf("Gmail edit not reconciled: %+v", record)
			}
			preview, err := svc.Sender(ctx, actor, legacy.ID)
			if err != nil || !preview.ImportedDraft || preview.Signature != "" || preview.Revision != "draft-message-2" || !slices.Equal(preview.Bcc, []string{"audit@cas.dev"}) || !slices.Equal(preview.Attachments, []string{"drawing.txt"}) {
				t.Fatalf("imported preview lost provider identity/content: %+v %v", preview, err)
			}
			if _, err := svc.SaveDraft(ctx, actor, legacy.ID, "stale overwrite", preview.Subject, "Email", preview.To, preview.Cc, "draft-message-1"); err == nil || provider.updates != 0 {
				t.Fatalf("a stale CRM editor overwrote Gmail: %v", err)
			}
			if outcome == "blocked save" {
				provider.putReady, provider.putResume = make(chan struct{}), make(chan struct{})
				saved := make(chan error, 1)
				go func() {
					_, err := svc.SaveDraft(ctx, actor, legacy.ID, "Edited in CRM", preview.Subject, "Email", preview.To, preview.Cc, preview.Revision)
					saved <- err
				}()
				select {
				case <-provider.putReady:
				case <-time.After(3 * time.Second):
					t.Fatal("provider update never started")
				}
				bounded, cancel := context.WithTimeout(ctx, time.Second)
				err := store.Atomically(bounded, func(tx storage.InteractionStore) error {
					if err := tx.LockWorkspace(bounded, actor.WorkspaceID); err != nil {
						return err
					}
					_, _, err := records.NewService(tx).Upsert(bounded, actor, records.SourceUser, records.Write{Object: records.FollowUps, Set: map[string][]string{"name": {"Independent work"}}})
					return err
				})
				cancel()
				close(provider.putResume)
				if err != nil {
					t.Fatalf("one blocked Gmail save prevented unrelated workspace work: %v", err)
				}
				if err := <-saved; err != nil {
					t.Fatal(err)
				}
				return
			}
			saved, err := svc.SaveDraft(ctx, actor, legacy.ID, "Edited in CRM", preview.Subject, "Email", preview.To, preview.Cc, preview.Revision)
			if err != nil || provider.updates != 1 || saved.Revision != "draft-message-3" || field(saved.Record, "draft") != "Edited in CRM" {
				t.Fatalf("CRM edit failed to update the same Gmail draft: %+v %v", saved, err)
			}
			if !strings.Contains(provider.raw, "YXR0YWNobWVudC1ieXRlcw==") || !strings.Contains(provider.raw, "Bcc: audit@cas.dev") {
				t.Fatalf("Gmail update dropped the attachment or Bcc: %s", provider.raw)
			}
			preview, err = svc.Sender(ctx, actor, legacy.ID)
			if err != nil {
				t.Fatal(err)
			}
			seen := followups.Seen{Confirmed: true, Draft: "Edited in CRM", Subject: preview.Subject, From: preview.From, To: preview.To, Cc: preview.Cc, Bcc: preview.Bcc, Revision: preview.Revision}
			switch outcome {
			case "sync during send":
				provider.afterSend = func() error {
					if err := svc.SyncGmailDrafts(ctx, connection, mailbox); err != nil {
						return err
					}
					_, err := svc.SaveDraft(ctx, actor, legacy.ID, "A new follow-up", "A new subject", "Email", preview.To, preview.Cc)
					return err
				}
				result, err := svc.Release(ctx, actor, legacy.ID, seen)
				if err != nil || field(result, "draft") != "A new follow-up" || provider.sends != 1 {
					t.Fatalf("send settlement repeated a sync completion or overwrote the next message: %+v %v", result, err)
				}
				return
			case "CRM send", "Bcc only":
				bad := seen
				bad.Bcc = nil
				if _, err := svc.Release(ctx, actor, legacy.ID, bad); err == nil || provider.sends != 0 {
					t.Fatalf("unreviewed Bcc was sent: %v", err)
				}
				if _, err := svc.Release(ctx, actor, legacy.ID, seen); err != nil {
					t.Fatal(err)
				}
				if _, err := svc.Release(ctx, actor, legacy.ID, seen); err == nil || provider.sends != 1 {
					t.Fatalf("provider draft sent twice: %d %v", provider.sends, err)
				}
			case "Gmail send":
				provider.consume()
			case "deleted", "delayed sent":
				provider.present = false
			case "stale send":
				provider.changeOnRaw = true
				if _, err := svc.Release(ctx, actor, legacy.ID, seen); err == nil || provider.sends != 0 {
					t.Fatalf("provider revision changed after confirmation: %d %v", provider.sends, err)
				}
				current, _ := crm.Get(ctx, actor, legacy.ID)
				if field(current, "draft_status") == records.DraftSending {
					t.Fatal("a pre-send revision refusal left a false uncertain send")
				}
				return
			}
			if err := svc.SyncGmailDrafts(ctx, connection, mailbox); err != nil {
				t.Fatal(err)
			}
			current, _ := crm.Get(ctx, actor, legacy.ID)
			want := "Done"
			if outcome == "deleted" || outcome == "delayed sent" {
				want = "Dismissed"
			}
			if field(current, "status") != want || field(current, "draft") != "" {
				t.Fatalf("provider %s not reconciled: %+v", outcome, current)
			}
			if _, err := svc.SaveDraft(ctx, actor, legacy.ID, "A new follow-up", "A new subject", "Email", preview.To, preview.Cc); err != nil {
				t.Fatalf("a terminal provider draft prevented a later message: %v", err)
			}
			if err := svc.SyncGmailDrafts(ctx, connection, mailbox); err != nil {
				t.Fatal(err)
			}
			newMessage, _ := crm.Get(ctx, actor, legacy.ID)
			if field(newMessage, "draft") != "A new follow-up" {
				t.Fatal("a historical provider draft cleared the newly composed message")
			}
			if outcome == "delayed sent" {
				provider.consume()
				if err := svc.SyncGmailDrafts(ctx, connection, mailbox); err != nil {
					t.Fatal(err)
				}
				draft, err := store.GmailDraft(ctx, actor.WorkspaceID, legacy.ID)
				if err != nil || draft.State != "sent" || draft.SentMessageID != "sent-message" {
					t.Fatalf("delayed SENT evidence was never reconciled: %+v %v", draft, err)
				}
				current, _ := crm.Get(ctx, actor, legacy.ID)
				if field(current, "draft") != "A new follow-up" {
					t.Fatal("late sent evidence overwrote a new composer")
				}
			}
			if provider.sent != "" {
				messages, err := mailbox.ThreadMessages(ctx, "thread")
				if err != nil || len(messages) != 1 {
					t.Fatalf("sent evidence: %+v %v", messages, err)
				}
				message := messages[0]
				known, err := convs.Known(ctx, actor.WorkspaceID)
				if err != nil {
					t.Fatal(err)
				}
				if err := convs.IngestEmail(ctx, known, interactions.EmailMessage{ConnectionID: connection.ID, UserID: connection.UserID, ProviderID: message.ID, ThreadID: message.ThreadID, MessageID: message.MessageID, From: interactions.Address(message.From), Subject: message.Subject, Date: message.Date, Labels: message.Labels}); err != nil {
					t.Fatal(err)
				}
				timeline, err := store.Timeline(ctx, storage.TimelineQuery{WorkspaceID: actor.WorkspaceID, RecordID: legacy.ID, Kinds: []string{interactions.Email}, Limit: 10})
				if err != nil || len(timeline) != 1 {
					t.Fatalf("actual sent message did not become linked history: %+v %v", timeline, err)
				}
			}
		})
	}
}

func field(record records.Record, attribute string) string {
	for _, field := range record.Fields {
		if field.Attribute == attribute && len(field.Values) > 0 {
			return field.Values[0].Text
		}
	}
	return ""
}

type draftMailbox struct {
	t                   *testing.T
	raw, sent           string
	revision            int
	present             bool
	updates, sends      int
	at                  time.Time
	changeOnRaw         bool
	putReady, putResume chan struct{}
	afterSend           func() error
}

func (d *draftMailbox) consume() {
	d.sent, d.present = d.raw, false
	d.sends++
}

func (d *draftMailbox) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/token", "/gmail/v1/users/me/profile":
		(&gmail{}).ServeHTTP(w, r)
	case "/gmail/v1/users/me/drafts":
		list := []any{}
		if d.present {
			list = append(list, map[string]any{"id": "stable-draft", "message": map[string]string{"id": fmt.Sprintf("draft-message-%d", d.revision), "threadId": "thread"}})
		}
		json.NewEncoder(w).Encode(map[string]any{"drafts": list})
	case "/gmail/v1/users/me/drafts/stable-draft":
		if !d.present {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPut {
			if d.putReady != nil {
				close(d.putReady)
				<-d.putResume
			}
			var in struct{ Message struct{ Raw string } }
			json.NewDecoder(r.Body).Decode(&in)
			raw, err := base64.RawURLEncoding.DecodeString(in.Message.Raw)
			if err != nil {
				d.t.Error(err)
			}
			d.raw, d.revision = string(raw), d.revision+1
			d.updates++
		}
		message := d.message(d.raw, fmt.Sprintf("draft-message-%d", d.revision), "DRAFT")
		if r.URL.Query().Get("format") == "raw" {
			if d.changeOnRaw {
				d.revision++
				message["id"] = fmt.Sprintf("draft-message-%d", d.revision)
			}
			message["raw"] = base64.RawURLEncoding.EncodeToString([]byte(d.raw))
			delete(message, "payload")
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "stable-draft", "message": message})
	case "/gmail/v1/users/me/drafts/send":
		var in struct {
			ID      string
			Message struct{ Raw string }
		}
		json.NewDecoder(r.Body).Decode(&in)
		if !d.present || in.ID != "stable-draft" {
			http.NotFound(w, r)
			return
		}
		raw, err := base64.RawURLEncoding.DecodeString(in.Message.Raw)
		if err != nil || string(raw) != d.raw {
			d.t.Errorf("send must retain reviewed MIME exactly: %v", err)
		}
		d.consume()
		if d.afterSend != nil {
			if err := d.afterSend(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		json.NewEncoder(w).Encode(map[string]string{"id": "sent-message", "threadId": "thread"})
	case "/gmail/v1/users/me/threads/thread":
		messages := []any{}
		if d.sent != "" {
			messages = append(messages, d.message(d.sent, "sent-message", "SENT"))
		}
		json.NewEncoder(w).Encode(map[string]any{"messages": messages})
	default:
		d.t.Errorf("unexpected provider endpoint %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}
}

func (d *draftMailbox) message(raw, id, label string) map[string]any {
	parsed, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		d.t.Fatal(err)
	}
	parsed.Header["Message-Id"] = []string{"<" + id + "@mail.gmail.com>"}
	payload := draftPayload(d.t, textproto.MIMEHeader(parsed.Header), parsed.Body)
	return map[string]any{"id": id, "threadId": "thread", "labelIds": []string{label}, "internalDate": fmt.Sprint(d.at.UnixMilli()), "payload": payload}
}

func draftPayload(t *testing.T, headers textproto.MIMEHeader, body io.Reader) map[string]any {
	t.Helper()
	kind, params, err := mime.ParseMediaType(headers.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	values := []map[string]string{}
	for name, entries := range headers {
		for _, value := range entries {
			decoded, err := new(mime.WordDecoder).DecodeHeader(value)
			if err != nil {
				t.Fatal(err)
			}
			values = append(values, map[string]string{"name": name, "value": decoded})
		}
	}
	result := map[string]any{"mimeType": kind, "headers": values}
	if strings.HasPrefix(kind, "multipart/") {
		reader := multipart.NewReader(body, params["boundary"])
		parts := []any{}
		for {
			part, err := reader.NextRawPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			parts = append(parts, draftPayload(t, part.Header, part))
		}
		result["parts"] = parts
	} else {
		if headers.Get("Content-Transfer-Encoding") == "base64" {
			body = base64.NewDecoder(base64.StdEncoding, body)
		}
		data, err := io.ReadAll(body)
		if err != nil {
			t.Fatal(err)
		}
		result["body"] = map[string]string{"data": base64.RawURLEncoding.EncodeToString(data)}
		_, disposition, _ := mime.ParseMediaType(headers.Get("Content-Disposition"))
		if name := disposition["filename"]; name != "" {
			result["filename"] = name
		}
	}
	return result
}

func draftMIME(body, subject string) string {
	return "From: Owner <owner@cas.dev>\r\nTo: jane@acme.com\r\nBcc: audit@cas.dev\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=mixed\r\n\r\n" +
		"--mixed\r\nContent-Type: multipart/alternative; boundary=alternative\r\n\r\n--alternative\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n" + base64.StdEncoding.EncodeToString([]byte(body)) +
		"\r\n--alternative\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n" + base64.StdEncoding.EncodeToString([]byte("<p><b>"+body+"</b></p>")) +
		"\r\n--alternative--\r\n--mixed\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=drawing.txt\r\nContent-Transfer-Encoding: base64\r\n\r\nYXR0YWNobWVudC1ieXRlcw==\r\n--mixed--\r\n"
}
