package postgres_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

func draftStore(t *testing.T) (*postgres.Store, auth.Actor, storage.Connection, string) {
	t.Helper()
	ctx := t.Context()
	store := postgrestest.New(t)
	owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "owner@cas.dev")
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	connection, err := store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: owner.WorkspaceID, UserID: owner.ID, Provider: "google", Account: owner.Email, RefreshToken: []byte("encrypted")})
	if err != nil {
		t.Fatal(err)
	}
	record, _, err := records.NewService(store).Upsert(ctx, actor, records.SourceUser, records.Write{Object: "follow_ups", Set: map[string][]string{"name": {"Review Gmail draft"}}})
	if err != nil {
		t.Fatal(err)
	}
	return store, actor, connection, record.ID
}

func TestGmailDraftRevisionsKeepOneBinding(t *testing.T) {
	ctx := t.Context()
	store, actor, connection, followUp := draftStore(t)
	draft := storage.GmailDraft{ConnectionID: connection.ID, DraftID: "r-provider-draft", MessageID: "revision-1", Subject: "A proposal", Body: "First version", State: "draft"}
	if err := store.UpsertGmailDraft(ctx, draft); err != nil {
		t.Fatal(err)
	}
	if err := store.BindGmailDraft(ctx, connection.ID, draft.DraftID, followUp); err != nil {
		t.Fatal(err)
	}
	draft.MessageID = "revision-2"
	draft.Body = "Reviewed version"
	draft.At = time.Now().UTC().Truncate(time.Microsecond)
	draft.Attachments = []string{"part.step", "drawing.pdf"}
	draft.To = []string{"david@apex.dev"}
	draft.Cc = []string{"colleague@cas.dev"}
	draft.Bcc = []string{"archive@cas.dev"}
	if err := store.UpsertGmailDraft(ctx, draft); err != nil {
		t.Fatal(err)
	}
	current, err := store.GmailDraft(ctx, actor.WorkspaceID, followUp)
	if err != nil || current.MessageID != draft.MessageID || current.Body != draft.Body || current.FollowUpID == nil || *current.FollowUpID != followUp || !slices.Equal(current.To, draft.To) || !slices.Equal(current.Bcc, draft.Bcc) || !current.At.Equal(draft.At) || !slices.Equal(current.Attachments, draft.Attachments) {
		t.Fatalf("draft edit lost its binding or content: %+v %v", current, err)
	}
	other, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "other@company.dev")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GmailDraft(ctx, other.WorkspaceID, followUp); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("draft escaped workspace: %v", err)
	}
	err = store.Atomically(ctx, func(tx storage.InteractionStore) error {
		duplicate := draft
		duplicate.DraftID = "another-provider-draft"
		if err := tx.UpsertGmailDraft(ctx, duplicate); err != nil {
			return err
		}
		return tx.BindGmailDraft(ctx, connection.ID, duplicate.DraftID, followUp)
	})
	if !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("two provider drafts shared one follow-up: %v", err)
	}
	rolledBack := errors.New("abort reconciliation")
	err = store.Atomically(ctx, func(tx storage.InteractionStore) error {
		draft.Body = "Rolled back version"
		if err := tx.UpsertGmailDraft(ctx, draft); err != nil {
			return err
		}
		return rolledBack
	})
	if !errors.Is(err, rolledBack) {
		t.Fatal(err)
	}
	current, err = store.GmailDraft(ctx, actor.WorkspaceID, followUp)
	if err != nil || current.Body != "Reviewed version" {
		t.Fatalf("partial reconciliation persisted: %+v %v", current, err)
	}
	draft.State = "sent"
	draft.SentMessageID = "sent-message"
	if err := store.UpsertGmailDraft(ctx, draft); err != nil {
		t.Fatal(err)
	}
	draft.State = "draft"
	draft.SentMessageID = ""
	if err := store.UpsertGmailDraft(ctx, draft); err != nil {
		t.Fatal(err)
	}
	current, err = store.GmailDraftByID(ctx, connection.ID, draft.DraftID)
	if err != nil || current.State != "sent" || current.SentMessageID != "sent-message" {
		t.Fatalf("stale draft refresh revived a sent message: %+v %v", current, err)
	}
	if pending, err := store.GmailDrafts(ctx, connection.ID); err != nil || len(pending) != 0 {
		t.Fatalf("terminal drafts kept polling Gmail: %+v %v", pending, err)
	}
}

func TestRepairDraftPreservesContentWithoutContactActivityUntilSent(t *testing.T) {
	ctx := t.Context()
	store, actor, connection, followUp := draftStore(t)
	before := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	after := before.Add(time.Minute)
	id, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: actor.WorkspaceID, ExternalID: "gmail-thread", ConnectionID: &connection.ID, At: after, Title: "A proposal"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddLink(ctx, id, followUp, "user"); err != nil {
		t.Fatal(err)
	}
	for _, participant := range []struct {
		address string
		triage  string
		role    string
	}{{connection.Account, "internal", "from"}, {"david@apex.dev", "pending", "to"}} {
		handle, err := store.UpsertHandle(ctx, storage.NewHandle{WorkspaceID: actor.WorkspaceID, Kind: "email", Value: participant.address, Triage: participant.triage})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.AddParticipant(ctx, id, handle.ID, participant.role); err != nil {
			t.Fatal(err)
		}
	}
	for _, message := range []struct {
		id string
		at time.Time
	}{{"earlier", before}, {"draft", after}} {
		if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: id, Kind: "message", ExternalID: message.id, ConnectionID: &connection.ID, ProviderID: &message.id, At: &message.at}); err != nil {
			t.Fatal(err)
		}
	}
	parts, err := store.Parts(ctx, []string{id})
	if err != nil {
		t.Fatal(err)
	}
	var draftPart int64
	for _, part := range parts {
		if part.ExternalID == "draft" {
			draftPart = part.ID
			if err := store.SetPartContent(ctx, part.ID, "Original draft body", "<p>Original draft body</p>"); err != nil {
				t.Fatal(err)
			}
		}
	}
	draft := storage.GmailDraft{ConnectionID: connection.ID, DraftID: "stable-draft", MessageID: "draft", State: "draft", Body: "Current draft body", At: after}
	if err := store.UpsertGmailDraft(ctx, draft); err != nil {
		t.Fatal(err)
	}
	if err := store.BindGmailDraft(ctx, connection.ID, draft.DraftID, followUp); err != nil {
		t.Fatal(err)
	}
	started, err := store.ClaimFollowUp(ctx, id, nil, &after)
	if err != nil || started == nil {
		t.Fatalf("initial follow-up claim: %v %v", started, err)
	}
	ids, err := store.RepairDraftMessage(ctx, connection.ID, "draft")
	if err != nil || !slices.Equal(ids, []string{id}) {
		t.Fatalf("repair affected wrong conversation: %v %v", ids, err)
	}
	parts, err = store.Parts(ctx, []string{id})
	if err != nil || len(parts) != 2 || !slices.ContainsFunc(parts, func(part storage.Part) bool {
		return part.ID == draftPart && part.Kind == "draft" && part.Content != nil && *part.Content == "Original draft body" && part.HTML != nil && *part.HTML == "<p>Original draft body</p>"
	}) {
		t.Fatalf("repair lost original draft content or identity: %+v %v", parts, err)
	}
	if drafts, err := store.InteractionDrafts(ctx, ids); err != nil || len(drafts) != 1 || drafts[0].InteractionID != id || drafts[0].Body != draft.Body || drafts[0].FollowUpID == nil || *drafts[0].FollowUpID != followUp {
		t.Fatalf("original interaction lost reviewable draft: %+v %v", drafts, err)
	}
	conversations, err := store.Interactions(ctx, actor.WorkspaceID, ids)
	if err != nil || len(conversations) != 1 || !conversations[0].StartedAt.Equal(before) || conversations[0].EndedAt == nil || !conversations[0].EndedAt.Equal(before) || conversations[0].DraftingState != "" || conversations[0].FollowedUpAt != nil {
		t.Fatalf("draft kept conversation active or claimed: %+v %v", conversations, err)
	}
	if _, err := store.RepairDraftMessage(ctx, connection.ID, "earlier"); err != nil {
		t.Fatal(err)
	}
	if links, err := store.Links(ctx, ids); err != nil || len(links) != 1 || links[0].RecordID != followUp {
		t.Fatalf("repair lost record links: %v %v", links, err)
	}
	if timeline, err := store.Timeline(ctx, storage.TimelineQuery{WorkspaceID: actor.WorkspaceID, RecordID: followUp, Kinds: []string{}, Limit: 20}); err != nil || len(timeline) != 0 {
		t.Fatalf("empty Gmail thread appears in timeline: %v %v", timeline, err)
	}
	if found, err := store.SearchInteractions(ctx, actor.WorkspaceID, "proposal", 20); err != nil || len(found) != 0 {
		t.Fatalf("empty Gmail thread appears in search: %v %v", found, err)
	}
	if activity, err := store.RecordActivity(ctx, actor.WorkspaceID, []string{followUp}); err != nil || len(activity) != 0 {
		t.Fatalf("draft counts as contact: %v %v", activity, err)
	}
	if engaged, err := store.EngagedHandles(ctx, actor.WorkspaceID, 5, true, false); err != nil || len(engaged) != 0 {
		t.Fatalf("unsent draft qualifies a contact: %v %v", engaged, err)
	}
	if handles, err := store.ListHandles(ctx, storage.HandleQuery{WorkspaceID: actor.WorkspaceID, Triage: "pending", Limit: 20}); err != nil || len(handles) != 1 || handles[0].Interactions != 0 {
		t.Fatalf("triage counts draft as contact: %v %v", handles, err)
	}
	if handles, err := store.UnassessedHandles(ctx, actor.WorkspaceID, 20); err != nil || len(handles) != 1 || len(handles[0].Titles) != 0 {
		t.Fatalf("draft title remains in triage context: %v %v", handles, err)
	}
	if pending, err := store.UnfetchedParts(ctx, connection.ID, 20); err != nil || len(pending) != 0 {
		t.Fatalf("draft revisions remain queued for message fetching: %+v %v", pending, err)
	}
	sentID := "sent-message"
	if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: id, Kind: "message", ExternalID: "draft", ConnectionID: &connection.ID, ProviderID: &sentID, At: &after}); err != nil {
		t.Fatal(err)
	}
	parts, err = store.Parts(ctx, []string{id})
	if err != nil || len(parts) != 2 || !slices.ContainsFunc(parts, func(part storage.Part) bool {
		return part.ID == draftPart && part.Kind == "message" && part.Content == nil && part.HTML == nil && part.ProviderID != nil && *part.ProviderID == sentID
	}) {
		t.Fatalf("sent provider identity retained old draft content: %+v %v", parts, err)
	}
	if pending, err := store.UnfetchedParts(ctx, connection.ID, 20); err != nil || len(pending) != 1 || pending[0].ProviderID != sentID {
		t.Fatalf("sent copy was not scheduled for actual content: %+v %v", pending, err)
	}
	if err := store.SetPartContent(ctx, draftPart, "Actual sent body", "<p>Actual sent body</p>"); err != nil {
		t.Fatal(err)
	}
	if candidates, err := store.FollowUpCandidates(ctx, actor.WorkspaceID, before.Add(-time.Hour), 20); err != nil || len(candidates) != 1 || candidates[0].ID != id {
		t.Fatalf("retained draft blocked follow-up assessment of sent message: %+v %v", candidates, err)
	}
	if timeline, err := store.Timeline(ctx, storage.TimelineQuery{WorkspaceID: actor.WorkspaceID, RecordID: followUp, Kinds: []string{}, Limit: 20}); err != nil || len(timeline) != 1 || timeline[0].ID != id {
		t.Fatalf("sent provider message failed to restore conversation: %v %v", timeline, err)
	}
	if engaged, err := store.EngagedHandles(ctx, actor.WorkspaceID, 5, true, false); err != nil || len(engaged) != 1 {
		t.Fatalf("sent provider message failed to qualify contact: %v %v", engaged, err)
	}
}

func TestDisconnectWithdrawsOnlyUnsentGmailDrafts(t *testing.T) {
	ctx := t.Context()
	store, actor, connection, followUp := draftStore(t)
	crm := records.NewService(store)
	recordIDs := make(map[string]string)
	for _, state := range []string{"draft", "sent", "unbound"} {
		id := ""
		if state == "draft" {
			id = followUp
		}
		record, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "follow_ups", RecordID: id, Set: map[string][]string{
			"name": {"Review " + state}, "draft": {state + " body"}, "subject": {state + " subject"}, "channel": {"Email"},
		}})
		if err != nil {
			t.Fatal(err)
		}
		recordIDs[state] = record.ID
		if state == "unbound" {
			continue
		}
		draft := storage.GmailDraft{ConnectionID: connection.ID, DraftID: state, MessageID: state, State: state, At: time.Now()}
		if err := store.UpsertGmailDraft(ctx, draft); err != nil {
			t.Fatal(err)
		}
		if err := store.BindGmailDraft(ctx, connection.ID, draft.DraftID, record.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.DeleteConnection(ctx, actor.UserID, connection.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("cross-workspace disconnect: %v", err)
	}
	if draft, err := store.GmailDraft(ctx, actor.WorkspaceID, followUp); err != nil || draft.State != "draft" {
		t.Fatalf("cross-workspace disconnect changed draft: %+v %v", draft, err)
	}
	if history, err := store.History(ctx, actor.WorkspaceID, followUp, 100); err != nil || slices.ContainsFunc(history, func(v storage.PastValue) bool {
		return v.Text != nil && *v.Text == "draft body" && v.ActiveUntil != nil
	}) {
		t.Fatalf("cross-workspace disconnect retired composer: %+v %v", history, err)
	}
	if err := store.DeleteConnection(ctx, actor.WorkspaceID, connection.ID); err != nil {
		t.Fatal(err)
	}
	for state, id := range recordIDs {
		record, err := crm.Get(ctx, actor, id)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, field := range record.Fields {
			if slices.Contains([]string{"draft", "subject", "draft_status"}, field.Attribute) {
				got[field.Attribute] = field.Values[0].Text
			}
		}
		want := map[string]string{}
		if state != "draft" {
			want = map[string]string{"draft": state + " body", "subject": state + " subject", "draft_status": "Draft"}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("disconnect composer %s: got %v, want %v", state, got, want)
		}
	}
	if history, err := store.History(ctx, actor.WorkspaceID, followUp, 100); err != nil || !slices.ContainsFunc(history, func(v storage.PastValue) bool {
		return v.Text != nil && *v.Text == "draft body" && v.ActiveUntil != nil
	}) {
		t.Fatalf("disconnect erased original draft history: %+v %v", history, err)
	}
	if drafts, err := store.GmailDrafts(ctx, connection.ID); err != nil || len(drafts) != 0 {
		t.Fatalf("disconnect retained provider state: %+v %v", drafts, err)
	}
}

func TestGmailDraftSentLinksFollowProviderIdentity(t *testing.T) {
	for _, order := range []string{"draft-first", "message-first", "recipient-first"} {
		t.Run(order, func(t *testing.T) {
			ctx := t.Context()
			store, actor, connection, followUp := draftStore(t)
			at := time.Now().Add(-time.Minute)
			draft := storage.GmailDraft{ConnectionID: connection.ID, DraftID: "draft", MessageID: "draft-message", ThreadID: "draft-thread", State: "draft", At: at}
			if err := store.UpsertGmailDraft(ctx, draft); err != nil {
				t.Fatal(err)
			}
			if err := store.BindGmailDraft(ctx, connection.ID, draft.DraftID, followUp); err != nil {
				t.Fatal(err)
			}
			id, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: actor.WorkspaceID, ExternalID: "sent-thread", ConnectionID: &connection.ID, At: at})
			if err != nil {
				t.Fatal(err)
			}
			draft.State = "sent"
			draft.SentMessageID = "sent-message"
			if order != "message-first" {
				if err := store.UpsertGmailDraft(ctx, draft); err != nil {
					t.Fatal(err)
				}
			}
			partConnection, providerID := connection.ID, draft.SentMessageID
			if order == "recipient-first" {
				recipient, err := store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: actor.WorkspaceID, UserID: actor.UserID, Provider: "google", Account: "recipient@cas.dev", RefreshToken: []byte("encrypted")})
				if err != nil {
					t.Fatal(err)
				}
				partConnection = recipient.ID
				providerID = "recipient-message"
			}
			if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: id, Kind: "message", ExternalID: "rfc-sent", ConnectionID: &partConnection, ProviderID: &providerID, At: &at}); err != nil {
				t.Fatal(err)
			}
			if order == "message-first" {
				if err := store.UpsertGmailDraft(ctx, draft); err != nil {
					t.Fatal(err)
				}
			} else if err := store.LinkSentGmailDrafts(ctx, connection.ID, draft.SentMessageID, id); err != nil {
				t.Fatal(err)
			}
			if err := store.Relink(ctx, []string{id}); err != nil {
				t.Fatal(err)
			}
			if links, err := store.Links(ctx, []string{id}); err != nil || len(links) != 1 || links[0].RecordID != followUp {
				t.Fatalf("sent message lost its originating follow-up: %+v %v", links, err)
			}
		})
	}
}

func TestGmailDraftLockProtectsFirstImportWithoutBlockingOtherDrafts(t *testing.T) {
	store, _, connection, _ := draftStore(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	held := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- store.Atomically(ctx, func(tx storage.InteractionStore) error {
			if err := tx.LockGmailDraft(ctx, connection.ID, "not-imported-yet"); err != nil {
				return err
			}
			close(held)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	select {
	case <-held:
	case err := <-finished:
		t.Fatalf("initial draft lock: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	defer func() {
		close(release)
		if err := <-finished; err != nil {
			t.Errorf("release draft lock: %v", err)
		}
	}()
	if err := store.Atomically(ctx, func(tx storage.InteractionStore) error {
		return tx.LockGmailDraft(ctx, connection.ID, "another-draft")
	}); err != nil {
		t.Fatalf("one draft blocked another: %v", err)
	}
	blocked, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	err := store.Atomically(blocked, func(tx storage.InteractionStore) error {
		return tx.LockGmailDraft(blocked, connection.ID, "not-imported-yet")
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("two imports acquired the same draft: %v", err)
	}
}
