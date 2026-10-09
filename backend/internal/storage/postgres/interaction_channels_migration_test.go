package postgres_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/lithammer/shortuuid/v4"
)

// Email threads become messages on the email channel with their Gmail link;
// logged chats sharing a thread link become one conversation that logging
// adds to; note citations stay with the note.
func TestInteractionChannelsMigration(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 47)
	workspace, user, connection, people, followUps, person := shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New()
	email, meeting, first, second, removed, note, recorded := shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New(), shortuuid.New()
	thread := "https://www.linkedin.com/messaging/thread/T1"
	for _, seed := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO workspaces(id, name) VALUES ($1, 'CAS')`, []any{workspace}},
		{`INSERT INTO users(id, workspace_id, name, email) VALUES ($1, $2, 'Owner', 'owner@cas.dev')`, []any{user, workspace}},
		{`INSERT INTO connections(id, workspace_id, user_id, provider, account, refresh_token) VALUES ($1, $2, $3, 'google', 'owner@cas.dev', 'sealed')`, []any{connection, workspace, user}},
		{`INSERT INTO sync_cursors(connection_id, stream, cursor) VALUES ($1, 'calendar', 'token'), ($1, 'gmail_history', '42')`, []any{connection}},
		{`INSERT INTO objects(id, workspace_id, slug, name) VALUES ($1, $2, 'people', 'People'), ($3, $2, 'follow_ups', 'Follow-ups')`, []any{people, workspace, followUps}},
		{`INSERT INTO attributes(id, object_id, slug, name, type) VALUES ($1, $2, 'name', 'Name', 'text')`, []any{shortuuid.New(), people}},
		{`INSERT INTO attributes(id, object_id, slug, name, type, options) VALUES ($1, $2, 'channel', 'Channel', 'select', $3)`, []any{shortuuid.New(), followUps, []string{"Email", "LinkedIn", "WhatsApp", "Pigeon"}}},
		{`INSERT INTO records(id, workspace_id, object_id) VALUES ($1, $2, $3)`, []any{person, workspace, people}},
		{`INSERT INTO interactions(id, workspace_id, kind, source, external_id, connection_id, user_id, title, started_at) VALUES
			($1, $3, 'email', 'gmail', 'thr1', $4, $5, 'Quote', '2026-10-01T09:00:00Z'),
			($2, $3, 'meeting', 'calendar', 'ev1', $4, $5, 'Review', '2026-10-01T10:00:00Z')`, []any{email, meeting, workspace, connection, user}},
		{`INSERT INTO parts(interaction_id, kind, external_id, connection_id, provider_id, at, content) VALUES ($1, 'message', 'm1@mail', $2, 'p1', '2026-10-01T09:00:00Z', 'Hello')`, []any{email, connection}},
		{`INSERT INTO interactions(id, workspace_id, kind, source, channel, external_id, user_id, title, started_at, date_only, provenance, skipped) VALUES
			($1, $4, 'message', 'manual', 'linkedin', 'linkedin:a', $5, 'LinkedIn message', '2026-10-01', true, 'Thread: ' || $6 || '/.', false),
			($2, $4, 'message', 'manual', 'linkedin', 'linkedin:b', $5, 'LinkedIn message', '2026-10-02', true, $6 || '/', false),
			($3, $4, 'message', 'manual', 'linkedin', 'linkedin:c', $5, 'LinkedIn message', '2026-09-30', true, $6, true)`, []any{first, second, removed, workspace, user, thread}},
		{`INSERT INTO parts(interaction_id, kind, external_id, author_name, recipients, direction, at, date_only, content, position) VALUES
			($1, 'message', 'body', 'Jim', '{Owner}', 'received', '2026-10-01', true, 'Great list.', 1),
			($2, 'message', 'body', 'Owner', '{Jim}', 'sent', '2026-10-02', true, 'Thanks for the message.', 1),
			($3, 'message', 'body', 'Jim', '{Owner}', 'received', '2026-09-30', true, 'Removed.', 1)`, []any{first, second, removed}},
		{`INSERT INTO links(interaction_id, record_id, source) VALUES ($1, $3, 'agent'), ($2, $3, 'agent')`, []any{first, second, person}},
		{`INSERT INTO interactions(id, workspace_id, kind, source, external_id, user_id, title, started_at, provenance) VALUES
			($1, $3, 'note', 'manual', 'research', $4, 'Research', '2026-10-03T09:00:00Z', 'Sources: https://a.example'),
			($2, $3, 'meeting', 'manual', 'granola:1', $4, 'Call', '2026-10-03T10:00:00Z', 'Imported from Granola: https://notes.granola.ai/d/1')`, []any{note, recorded, workspace, user}},
		{`INSERT INTO parts(interaction_id, kind, external_id, author_name, at, content) VALUES ($1, 'note', 'body', 'Agent', '2026-10-03T09:00:00Z', 'Research.'), ($2, 'note', 'body', 'Agent', '2026-10-03T10:00:00Z', 'Notes.')`, []any{note, recorded}},
	} {
		if _, err := db.ExecContext(ctx, seed.query, seed.args...); err != nil {
			t.Fatal(err)
		}
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	svc := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: records.NewService(store)})
	actor := auth.Actor{UserID: user, WorkspaceID: workspace}
	got, err := svc.Get(ctx, actor, email)
	if err != nil || got.Kind != interactions.Message || got.Channel != "email" || got.URL != "https://mail.google.com/mail/u/owner@cas.dev/#all/thr1" {
		t.Fatalf("email thread: %+v %v", got, err)
	}
	var streams []string
	rows, err := db.QueryContext(ctx, `SELECT stream FROM sync_cursors WHERE connection_id = $1`, connection)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var stream string
		if err := rows.Scan(&stream); err != nil {
			t.Fatal(err)
		}
		streams = append(streams, stream)
	}
	if !slices.Equal(streams, []string{"gmail_history"}) {
		t.Fatalf("only the calendar resyncs, for its event links: %v", streams)
	}
	chat, err := svc.Get(ctx, actor, first)
	if err != nil || chat.URL != thread || len(chat.Messages) != 2 || chat.Messages[1].Text != "Thanks for the message." || len(chat.Records) != 1 || chat.StartedAt != "2026-10-01" || chat.EndedAt == nil {
		t.Fatalf("chats sharing a thread link merge: %+v %v", chat, err)
	}
	if _, err := svc.Get(ctx, actor, second); err == nil {
		t.Fatal("the merged duplicate remains")
	}
	var skipped bool
	if err := db.QueryRowContext(ctx, `SELECT skipped FROM interactions WHERE id = $1`, removed).Scan(&skipped); err != nil || !skipped {
		t.Fatalf("a removed chat must stay removed: %v %v", skipped, err)
	}
	reply := interactions.Entry{Kind: interactions.Message, Channel: "linkedin", URL: thread + "/", At: "2026-10-04", Sender: "Jim", Recipients: []string{"Owner"}, Direction: "received", Text: "Great list."}
	if _, err := svc.Log(ctx, actor, reply); err != nil {
		t.Fatal(err)
	}
	reply.At, reply.Text = "2026-10-01", "Great list."
	again, err := svc.Log(ctx, actor, reply)
	if err != nil || again.ID != first || len(again.Messages) != 3 {
		t.Fatalf("logging the thread adds to the merged chat without repeating migrated messages: %+v %v", again, err)
	}
	if got, err := svc.Get(ctx, actor, note); err != nil || got.Text != "Research.\n\nSources: https://a.example" {
		t.Fatalf("note citations: %+v %v", got, err)
	}
	if got, err := svc.Get(ctx, actor, recorded); err != nil || got.URL != "https://notes.granola.ai/d/1" {
		t.Fatalf("recorded meeting link: %+v %v", got, err)
	}
	var raw []byte
	var options []string
	if err := db.QueryRowContext(ctx, `SELECT array_to_json(options) FROM attributes WHERE object_id = $1 AND slug = 'channel'`, followUps).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &options); err != nil || !slices.Equal(options, []string{"Email", "LinkedIn", "WhatsApp", "X", "Telegram", "SMS", "Pigeon"}) {
		t.Fatalf("follow-up channels: %v %v", options, err)
	}
}
