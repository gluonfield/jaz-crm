package postgres_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	datamigrations "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/migrations"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lithammer/shortuuid/v4"
	"github.com/pressly/goose/v3"
)

// legacy returns a database migrated up to version, as one deployed then,
// and its URL, which postgres.Open migrates the rest of the way.
func legacy(t *testing.T, version int64) (*sql.DB, string) {
	t.Helper()
	ctx := context.Background()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		base = postgrestest.DefaultURL
	}
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := "jazcrm_test_" + shortuuid.New()
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
		_ = admin.Close(ctx)
	})
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"), goose.WithDisableGlobalRegistry(true), goose.WithGoMigrations(datamigrations.DealFollowups, datamigrations.FollowUps, datamigrations.ChaseFilter))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, version); err != nil {
		t.Fatal(err)
	}
	return db, u.String()
}

func TestMessageMigration(t *testing.T) {
	ctx := context.Background()
	db, dsn := legacy(t, 17)
	var workspace, user, object, record, messageID, noteID, otherNoteID, callID string
	for _, seed := range []struct {
		query string
		args  []any
		id    *string
	}{
		{"INSERT INTO workspaces(name) VALUES('CAS') RETURNING id", nil, &workspace},
		{"INSERT INTO users(workspace_id,name,email) VALUES($1,'August','august@example.com') RETURNING id", []any{&workspace}, &user},
	} {
		if err := db.QueryRowContext(ctx, seed.query, seed.args...).Scan(seed.id); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.QueryRowContext(ctx, "INSERT INTO objects(workspace_id,slug,name) VALUES($1,'people','People') RETURNING id", workspace).Scan(&object); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "INSERT INTO records(workspace_id,object_id) VALUES($1,$2) RETURNING id", workspace, object).Scan(&record); err != nil {
		t.Fatal(err)
	}
	preamble := "Imported through the old note/transcript schema."
	captured := "Channel: LinkedIn\nSender: Ada\nRecipient: August\nMessage date displayed by LinkedIn: 2026-09-20; time and timezone unavailable.\nVerified partial message (verbatim preview, ending at capture cutoff):\nA connected factory preview\n\nSource: capture.jsonl\nThis is a truncated incoming-message preview, not the full exchange."
	for _, entry := range []struct {
		title string
		id    *string
		parts []struct{ kind, author, text string }
	}{
		{"LinkedIn message excerpt — Ada — 2026-09-20", &messageID, []struct{ kind, author, text string }{{"note", "", preamble}, {"transcript", "", captured}}},
		{"Robotics manufacturing expertise", &noteID, []struct{ kind, author, text string }{{"note", "", "Hardware operator.\nSources: https://example.com/bio"}}},
		{"Other note", &otherNoteID, []struct{ kind, author, text string }{{"note", "", "Commentary"}, {"transcript", "Ada", "Original speech"}}},
	} {
		if err := db.QueryRowContext(ctx, "INSERT INTO interactions(workspace_id,user_id,kind,source,external_id,title,started_at) VALUES($1,$2,'note','manual',$3,$3,'2026-10-01T10:00:00Z') RETURNING id", workspace, user, entry.title).Scan(entry.id); err != nil {
			t.Fatal(err)
		}
		for _, part := range entry.parts {
			if _, err := db.ExecContext(ctx, "INSERT INTO parts(interaction_id,kind,external_id,author_name,at,content) VALUES($1,$2,$2,$3,'2026-10-01T10:00:00Z',$4)", *entry.id, part.kind, part.author, part.text); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO links(interaction_id,record_id,source) VALUES($1,$2,'agent')", *entry.id, record); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.QueryRowContext(ctx, "INSERT INTO interactions(workspace_id,user_id,kind,source,external_id,started_at) VALUES($1,$2,'call','webhook','legacy-call','2026-09-21T12:00:00Z') RETURNING id", workspace, user).Scan(&callID); err != nil {
		t.Fatal(err)
	}
	for _, turn := range []struct{ speaker, at string }{{"Ada", "2026-09-21T12:02:00Z"}, {"August", "2026-09-21T12:01:00Z"}} {
		if _, err := db.ExecContext(ctx, "INSERT INTO parts(interaction_id,kind,external_id,author_name,at,content) VALUES($1,'transcript',$2,$2,$3,$2)", callID, turn.speaker, turn.at); err != nil {
			t.Fatal(err)
		}
	}
	for pass := range 2 {
		store, err := postgres.Open(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		crm := records.NewService(store)
		svc := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
		actor := auth.Actor{UserID: user, WorkspaceID: workspace}
		message, err := svc.Get(ctx, actor, messageID)
		if err != nil || message.Kind != "message" || message.Channel != "linkedin" || message.StartedAt != "2026-09-20" || len(message.Messages) != 1 || len(message.Records) != 1 || message.Records[0].ID != record {
			t.Fatalf("migrated message, pass %d: %+v %v", pass, message, err)
		}
		part := message.Messages[0]
		if part.Sender != "Ada" || part.At != "2026-09-20" || len(part.Recipients) != 1 || part.Recipients[0] != "August" || part.Text != "A connected factory preview" || !part.Partial || part.Direction != "received" || !strings.Contains(message.Provenance, captured) || !strings.Contains(message.Provenance, preamble) {
			t.Fatalf("migration lost message metadata or provenance: %+v", message)
		}
		note, err := svc.Get(ctx, actor, noteID)
		if err != nil || note.Text != "Hardware operator." || note.Provenance != "https://example.com/bio" || note.Author != "Imported" || len(note.Messages) != 0 || len(note.Transcript) != 0 {
			t.Fatalf("migrated research note: %+v %v", note, err)
		}
		other, err := svc.Get(ctx, actor, otherNoteID)
		if err != nil || other.Text != "Commentary\n\nAda: Original speech" || other.Author != "August" {
			t.Fatalf("legacy note content or author lost: %+v %v", other, err)
		}
		call, err := svc.Get(ctx, actor, callID)
		if err != nil || len(call.Transcript) != 2 || call.Transcript[0].Speaker != "Ada" || call.Transcript[1].Speaker != "August" || call.Transcript[0].At != "2026-09-21T12:02:00Z" || call.Transcript[1].At != "2026-09-21T12:01:00Z" {
			t.Fatalf("legacy transcript order or recorded times lost: %+v %v", call, err)
		}
		activity, err := svc.Activities(ctx, actor, []string{record})
		if err != nil || activity[record].Interactions != 1 {
			t.Fatalf("migration still counts notes as contact: %+v %v", activity, err)
		}
		store.Close()
	}
}
