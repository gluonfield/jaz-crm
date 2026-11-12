package followups_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/internal/connections"
	"github.com/gluonfield/jaz-crm/internal/followups"
	"github.com/gluonfield/jaz-crm/internal/google"
	"github.com/gluonfield/jaz-crm/internal/storage"
	"github.com/gluonfield/jaz-crm/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/internal/workspaces"
	"github.com/gluonfield/jaz-crm/records"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGmailDraftSyncPreservesTrashedFollowUps(t *testing.T) {
	for _, outcome := range []string{"edited", "missing", "reappeared"} {
		t.Run(outcome, func(t *testing.T) {
			ctx := t.Context()
			dsn := postgrestest.Database(t)
			store := postgrestest.Open(t, dsn)
			pool, err := pgxpool.New(ctx, dsn)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(pool.Close)
			owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "owner@cas.dev")
			if err != nil {
				t.Fatal(err)
			}
			actor := records.Actor{MemberID: owner.ID, WorkspaceID: owner.WorkspaceID}
			provider := map[string]*draftMailbox{}
			for _, id := range []string{"1-trashed", "2-active"} {
				provider[id] = &draftMailbox{t: t, raw: draftMIME("Original "+id, id), revision: 1, present: true, at: time.Now().Add(-time.Minute)}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/token", "/gmail/v1/users/me/profile":
					(&gmail{}).ServeHTTP(w, r)
				case "/gmail/v1/users/me/drafts":
					var list []any
					for _, id := range []string{"1-trashed", "2-active", "3-new"} {
						if draft := provider[id]; draft != nil && draft.present {
							list = append(list, map[string]any{"id": id, "message": map[string]string{"id": fmt.Sprintf("%s-%d", id, draft.revision)}})
						}
					}
					json.NewEncoder(w).Encode(map[string]any{"drafts": list})
				default:
					if id, ok := strings.CutPrefix(r.URL.Path, "/gmail/v1/users/me/drafts/"); ok {
						draft := provider[id]
						message := draft.message(draft.raw, fmt.Sprintf("%s-%d", id, draft.revision), "DRAFT")
						message["threadId"] = "thread-" + id
						json.NewEncoder(w).Encode(map[string]any{"id": id, "message": message})
						return
					}
					if strings.HasPrefix(r.URL.Path, "/gmail/v1/users/me/threads/") {
						json.NewEncoder(w).Encode(map[string]any{"messages": []any{}})
						return
					}
					t.Errorf("unexpected provider request %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
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
			if err := svc.SyncGmailDrafts(ctx, connection, mailbox); err != nil {
				t.Fatal(err)
			}
			original, err := store.GmailDraftByID(ctx, connection.ID, "1-trashed")
			if err != nil || original.FollowUpID == nil {
				t.Fatalf("original draft: %+v (%v)", original, err)
			}
			if outcome == "reappeared" {
				if _, err := pool.Exec(ctx, "UPDATE gmail_drafts SET state = 'missing', state_changed_at = now() - interval '2 days' WHERE connection_id = $1 AND draft_id = $2", connection.ID, original.DraftID); err != nil {
					t.Fatal(err)
				}
				original, err = store.GmailDraftByID(ctx, connection.ID, original.DraftID)
				if err != nil {
					t.Fatal(err)
				}
				pending, err := store.GmailDrafts(ctx, connection.ID)
				if err != nil || slices.ContainsFunc(pending, func(d storage.GmailDraft) bool { return d.DraftID == original.DraftID }) {
					t.Fatalf("old missing snapshot still selected for reconciliation: %+v (%v)", pending, err)
				}
			}
			history, err := crm.History(ctx, actor, *original.FollowUpID)
			if err != nil {
				t.Fatal(err)
			}
			if err := crm.Delete(ctx, actor, *original.FollowUpID); err != nil {
				t.Fatal(err)
			}
			if outcome != "reappeared" {
				provider["1-trashed"].revision++
				provider["1-trashed"].raw = draftMIME("Edited while trashed", "Changed subject")
			}
			provider["1-trashed"].present = outcome != "missing"
			provider["2-active"].revision++
			provider["2-active"].raw = draftMIME("Valid edited draft", "Changed valid subject")
			provider["3-new"] = &draftMailbox{t: t, raw: draftMIME("New valid draft", "New subject"), revision: 1, present: true, at: time.Now()}
			for range 2 {
				if err := svc.SyncGmailDrafts(ctx, connection, mailbox); err != nil {
					t.Fatalf("trashed binding aborted mailbox sync: %v", err)
				}
			}
			retained, err := store.GmailDraftByID(ctx, connection.ID, "1-trashed")
			if err != nil || !reflect.DeepEqual(retained, original) {
				t.Fatalf("trashed draft was mutated or detached: %+v (%v)", retained, err)
			}
			for id, want := range map[string]string{"2-active": "Valid edited draft", "3-new": "New valid draft"} {
				draft, err := store.GmailDraftByID(ctx, connection.ID, id)
				if err != nil || draft.FollowUpID == nil || draft.Body != want {
					t.Fatalf("valid draft did not flow: %+v (%v)", draft, err)
				}
				record, err := crm.Get(ctx, actor, *draft.FollowUpID)
				if err != nil || field(record, "draft") != want {
					t.Fatalf("valid follow-up did not reconcile: %+v (%v)", record, err)
				}
			}
			active, _, err := crm.Search(ctx, actor, records.Search{Object: records.FollowUps})
			if err != nil || len(active) != 2 {
				t.Fatalf("sync recreated trashed follow-up: %+v (%v)", active, err)
			}
			trash, err := crm.Trash(ctx, actor)
			if err != nil || len(trash) != 1 || trash[0].ID != *original.FollowUpID {
				t.Fatalf("sync lost the trashed follow-up: %+v (%v)", trash, err)
			}
			if err := crm.Restore(ctx, actor, *original.FollowUpID); err != nil {
				t.Fatal(err)
			}
			restoredHistory, err := crm.History(ctx, actor, *original.FollowUpID)
			if err != nil || !slices.Equal(history, restoredHistory) {
				t.Fatalf("sync rewrote retained history: %+v (%v)", restoredHistory, err)
			}
			if err := svc.SyncGmailDrafts(ctx, connection, mailbox); err != nil {
				t.Fatalf("restored follow-up did not rejoin sync: %v", err)
			}
			reconciled, err := store.GmailDraftByID(ctx, connection.ID, "1-trashed")
			if err != nil || reconciled.FollowUpID == nil || *reconciled.FollowUpID != *original.FollowUpID {
				t.Fatalf("restored draft lost binding: %+v (%v)", reconciled, err)
			}
			if outcome == "edited" && (reconciled.Body != "Edited while trashed" || reconciled.State != "draft") || outcome == "missing" && reconciled.State != "missing" || outcome == "reappeared" && reconciled.State != "draft" {
				t.Fatalf("restored provider state did not reconcile: %+v", reconciled)
			}
		})
	}
}
