package postgres_test

import (
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
)

func TestEmailSubjectMigration(t *testing.T) {
	ctx := t.Context()
	db, dsn := legacy(t, 35)
	var workspace, object string
	if err := db.QueryRowContext(ctx, "INSERT INTO workspaces(name) VALUES('Subjects') RETURNING id").Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "INSERT INTO objects(workspace_id,slug,name) VALUES($1,'follow_ups','Follow-ups') RETURNING id", workspace).Scan(&object); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"draft", "subject", "channel", "draft_status"} {
		if _, err := db.ExecContext(ctx, "INSERT INTO attributes(object_id,slug,name,type) VALUES($1,$2,$2,'text')", object, field); err != nil {
			t.Fatal(err)
		}
	}
	type example struct {
		channel, status, subject, body, wantSubject, wantBody, wantStatus, id string
	}
	cases := []example{
		{channel: "Email", status: "Approved", body: "Subject: Demo feedback\n\nHi Dan,\nHere is the demo.", wantSubject: "Demo feedback", wantBody: "Hi Dan,\nHere is the demo.", wantStatus: "Draft"},
		{channel: "Email", status: "Draft", body: "Subject: Unicode café\r\n\r\nHello!", wantSubject: "Unicode café", wantBody: "Hello!", wantStatus: "Draft"},
		{channel: "Email", status: "Sending", body: "Subject: In flight\n\nHello!", wantBody: "Subject: In flight\n\nHello!", wantStatus: "Sending"},
		{channel: "LinkedIn", status: "Approved", body: "Subject: Quoted text\n\nHello!", wantBody: "Subject: Quoted text\n\nHello!", wantStatus: "Approved"},
		{channel: "Email", status: "Draft", subject: "Already explicit", body: "Subject: Quoted text\n\nHello!", wantSubject: "Already explicit", wantBody: "Subject: Quoted text\n\nHello!", wantStatus: "Draft"},
		{channel: "Email", status: "Draft", body: "Hello!\nSubject: is just text here.", wantBody: "Hello!\nSubject: is just text here.", wantStatus: "Draft"},
	}
	for i := range cases {
		c := &cases[i]
		if err := db.QueryRowContext(ctx, "INSERT INTO records(workspace_id,object_id) VALUES($1,$2) RETURNING id", workspace, object).Scan(&c.id); err != nil {
			t.Fatal(err)
		}
		for field, value := range map[string]string{"draft": c.body, "subject": c.subject, "channel": c.channel, "draft_status": c.status} {
			if value != "" {
				if _, err := db.ExecContext(ctx, "INSERT INTO record_values(record_id,attribute_id,text,source) SELECT $1,id,$2,'user' FROM attributes WHERE object_id=$3 AND slug=$4", c.id, value, object, field); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	for _, c := range cases {
		for field, want := range map[string]string{"draft": c.wantBody, "subject": c.wantSubject, "draft_status": c.wantStatus} {
			var got string
			if err := db.QueryRowContext(ctx, "SELECT coalesce((SELECT v.text FROM record_values v JOIN attributes a ON a.id=v.attribute_id WHERE v.record_id=$1 AND a.slug=$2 AND v.active_until IS NULL),'')", c.id, field).Scan(&got); err != nil || got != want {
				t.Fatalf("%s %s: got %q, want %q: %v", c.channel, field, got, want, err)
			}
		}
		var originals int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM record_values v JOIN attributes a ON a.id=v.attribute_id WHERE v.record_id=$1 AND a.slug='draft' AND v.text=$2 AND v.source='user'", c.id, c.body).Scan(&originals); err != nil || originals != 1 {
			t.Fatalf("lost original draft/source: %d %v", originals, err)
		}
		var timestamps int
		if err := db.QueryRowContext(ctx, "SELECT count(distinct v.active_from) FROM record_values v JOIN attributes a ON a.id=v.attribute_id WHERE v.record_id=$1 AND a.slug='draft'", c.id).Scan(&timestamps); err != nil || timestamps != 1 {
			t.Fatalf("migration must not make an old draft look newly reviewed: %d %v", timestamps, err)
		}
	}
}
