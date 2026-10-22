// Package embed is a host program embedding the CRM through its public API,
// in a module of its own so it sees only what other programs see.
package embed

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	crm "github.com/gluonfield/jaz-crm"
	"github.com/gluonfield/jaz-crm/errs"
	"github.com/gluonfield/jaz-crm/interactions"
	"github.com/gluonfield/jaz-crm/records"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEmbeddedCRM(t *testing.T) {
	ctx := context.Background()
	pool := scratch(t)
	exec(t, pool,
		`CREATE TABLE users (id text PRIMARY KEY, email text NOT NULL)`,
		`INSERT INTO users VALUES ('u-1', 'ada@host.test'), ('u-2', 'bob@host.test')`,
		`CREATE TABLE goose_db_version (id serial PRIMARY KEY, version_id bigint NOT NULL, is_applied boolean NOT NULL)`,
		`INSERT INTO goose_db_version (version_id, is_applied) VALUES (0, true), (7, true)`,
	)
	hostPaths := searchPaths(t, pool)

	if _, err := crm.Open(ctx, pool, crm.Options{Schema: "crm"}); err == nil {
		t.Fatal("opened a schema that was never migrated")
	}
	if count(t, pool, `SELECT count(*) FROM pg_namespace WHERE nspname = 'crm'`) != 0 {
		t.Fatal("Open changed the database")
	}
	for range 2 {
		if err := crm.Migrate(ctx, pool, "crm"); err != nil {
			t.Fatal(err)
		}
	}
	c, err := crm.Open(ctx, pool, crm.Options{Schema: "crm"})
	if err != nil {
		t.Fatal(err)
	}

	ada := crm.Member{Principal: crm.Principal{Issuer: "host", Subject: "u-1"}, Name: "Ada", Email: "ada@host.test"}
	bob := crm.Member{Principal: crm.Principal{Issuer: "host", Subject: "u-2"}, Name: "Bob", Email: "bob@host.test"}
	owner, err := c.CreateWorkspace(ctx, "Acme Manufacturing", ada)
	if err != nil || owner.MemberID == "" || owner.MemberID == "u-1" {
		t.Fatalf("owner %+v: %v", owner, err)
	}
	member, err := c.AddMember(ctx, owner.WorkspaceID, bob)
	if err != nil {
		t.Fatal(err)
	}
	renamed := bob
	renamed.Email = "robert@host.test"
	if again, err := c.AddMember(ctx, owner.WorkspaceID, renamed); err != nil || again != member {
		t.Fatalf("adding bob again under a new email gave %+v, want %+v: %v", again, member, err)
	}
	if acting, err := c.ActAs(ctx, bob.Principal, owner.WorkspaceID); err != nil || acting != member {
		t.Fatalf("acting as bob gave %+v, want %+v: %v", acting, member, err)
	}
	// Callers adding one person at once agree on the member. Opening the
	// connections first and repeating the race keep a caller from finishing
	// before the others start.
	conns := make([]*pgxpool.Conn, 4)
	for i := range conns {
		if conns[i], err = pool.Acquire(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for _, conn := range conns {
		conn.Release()
	}
	for round := range 5 {
		person := crm.Member{Principal: crm.Principal{Issuer: "host", Subject: fmt.Sprint("new-", round)}, Name: "New", Email: fmt.Sprint("new", round, "@host.test")}
		joined := make([]records.Actor, len(conns))
		failures := make([]error, len(conns))
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range joined {
			wg.Go(func() {
				<-start
				joined[i], failures[i] = c.AddMember(ctx, owner.WorkspaceID, person)
			})
		}
		close(start)
		wg.Wait()
		for i := range joined {
			if failures[i] != nil || joined[i] != joined[0] {
				t.Fatalf("concurrent adds of one person disagree: %+v %v", joined, failures)
			}
		}
	}
	impostor := crm.Member{Principal: crm.Principal{Issuer: "host", Subject: "u-3"}, Name: "Eve", Email: "bob@host.test"}
	if _, err := c.AddMember(ctx, owner.WorkspaceID, impostor); !errors.As(err, new(errs.Invalid)) {
		t.Fatalf("a second person took bob's email: %v", err)
	}
	if _, err := c.ActAs(ctx, crm.Principal{Issuer: "email", Subject: "bob@host.test"}, owner.WorkspaceID); !errors.As(err, new(errs.Invalid)) || errors.Is(err, crm.ErrNotMember) {
		t.Fatalf("acted as a person proven by email: %v", err)
	}

	company, _, err := c.Records.Upsert(ctx, member, member.Source(), records.Write{
		Object: "companies",
		Set:    map[string][]string{"name": {"Acme"}, "domains": {"acme.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	agent := member
	agent.Agent = true
	same, skipped, err := c.Records.Upsert(ctx, agent, agent.Source(), records.Write{
		Object: "companies",
		Set:    map[string][]string{"domains": {"acme.com"}, "name": {"ACME Inc"}, "description": {"Presses"}},
	})
	if err != nil || same.ID != company.ID || len(skipped) != 1 || skipped[0].Attribute != "name" {
		t.Fatalf("agent write matched %s, skipped %+v: %v", same.ID, skipped, err)
	}
	if _, _, err := c.Records.Upsert(ctx, member, member.Source(), records.Write{Object: "companies", RecordID: company.ID, Set: map[string][]string{"name": {"Acme Corp"}}}); err != nil {
		t.Fatal(err)
	}
	read, err := c.Records.Get(ctx, owner, company.ID)
	if err != nil || text(read, "name") != "Acme Corp" || text(read, "description") != "Presses" {
		t.Fatalf("read %+v: %v", read.Fields, err)
	}
	history, err := c.Records.History(ctx, owner, company.ID)
	if err != nil {
		t.Fatal(err)
	}
	var names, agentEdits []string
	for _, change := range history {
		if change.Attribute == "name" && change.Source == records.SourceUser && change.Actor == bob.Name {
			names = append(names, change.Value.Text)
		}
		if change.Source == records.SourceAgent && change.Actor == bob.Name {
			agentEdits = append(agentEdits, change.Attribute)
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"Acme", "Acme Corp"}) || !slices.Contains(agentEdits, "description") {
		t.Fatalf("history lost who wrote what: names %v, agent edits %v", names, agentEdits)
	}

	note, err := c.Interactions.Log(ctx, member, interactions.Entry{Kind: "note", Text: "Wants a quote for 5,000 brackets", Records: []string{company.ID}})
	if err != nil {
		t.Fatal(err)
	}
	timeline, err := c.Interactions.Timeline(ctx, owner, company.ID, nil, "", false, 10)
	if err != nil || len(timeline) != 1 || timeline[0].ID != note.ID {
		t.Fatalf("timeline %+v: %v", timeline, err)
	}

	eve := crm.Member{Principal: crm.Principal{Issuer: "host", Subject: "u-9"}, Name: "Eve", Email: "eve@rival.test"}
	rival, err := c.CreateWorkspace(ctx, "Rival", eve)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ActAs(ctx, eve.Principal, owner.WorkspaceID); !errors.Is(err, crm.ErrNotMember) {
		t.Fatalf("eve acted in a workspace she is not a member of: %v", err)
	}
	if got, err := c.Records.Get(ctx, rival, company.ID); err == nil {
		t.Fatalf("rival read %s", got.ID)
	}
	if _, _, err := c.Records.Upsert(ctx, rival, rival.Source(), records.Write{Object: "companies", RecordID: company.ID, Set: map[string][]string{"name": {"Taken"}}}); err == nil {
		t.Fatal("rival wrote to another workspace's record")
	}
	if found, _, err := c.Records.Search(ctx, rival, records.Search{Object: "companies", Query: "acme"}); err != nil || len(found) != 0 {
		t.Fatalf("rival found %d companies: %v", len(found), err)
	}
	if _, err := c.Interactions.Log(ctx, rival, interactions.Entry{Kind: "note", Text: "Theirs", Records: []string{company.ID}}); err == nil {
		t.Fatal("rival linked a note to another workspace's record")
	}

	if n := count(t, pool, `SELECT count(*) FROM users WHERE id IN ('u-1', 'u-2')`); n != 2 {
		t.Fatalf("host users changed: %d", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'users'`); n != 2 {
		t.Fatalf("host users table has %d columns", n)
	}
	if n := count(t, pool, `SELECT max(version_id) FROM goose_db_version`); n != 7 {
		t.Fatalf("host migration history moved to %d", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename IN ('records', 'workspaces', 'interactions')`); n != 0 {
		t.Fatalf("%d CRM tables in public", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM crm.goose_db_version`); n < 2 {
		t.Fatalf("CRM migration history has %d rows", n)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("the CRM closed the host's pool: %v", err)
	}
	if paths := searchPaths(t, pool); !slices.Equal(paths, hostPaths) {
		t.Fatalf("host connections' search_path became %v, was %v", paths, hostPaths)
	}

	exec(t, pool, `DELETE FROM crm.goose_db_version WHERE version_id = (SELECT max(version_id) FROM crm.goose_db_version)`)
	if _, err := crm.Open(ctx, pool, crm.Options{Schema: "crm"}); err == nil {
		t.Fatal("opened a schema behind this version of the CRM")
	}
}

// scratch is a database of its own for the test, on the compose Postgres
// unless TEST_DATABASE_URL names another.
func scratch(t *testing.T) *pgxpool.Pool {
	ctx := context.Background()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		base = "postgres://jazcrm:jazcrm@localhost:55532/jazcrm?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("postgres unavailable at %s (start it with `docker compose up -d postgres`): %v", base, err)
	}
	name := "jazcrm_embed_" + strings.ToLower(rand.Text())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func exec(t *testing.T, pool *pgxpool.Pool, statements ...string) {
	for _, sql := range statements {
		if _, err := pool.Exec(context.Background(), sql); err != nil {
			t.Fatal(err)
		}
	}
}

func count(t *testing.T, pool *pgxpool.Pool, sql string) int {
	var n int
	if err := pool.QueryRow(context.Background(), sql).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// searchPaths returns the distinct search_path settings of the pool's idle
// connections, which are all of them between calls.
func searchPaths(t *testing.T, pool *pgxpool.Pool) []string {
	ctx := context.Background()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, conn := range pool.AcquireAllIdle(ctx) {
		var path string
		err := conn.QueryRow(ctx, "SHOW search_path").Scan(&path)
		conn.Release()
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	return paths
}

func text(r records.Record, attribute string) string {
	for _, f := range r.Fields {
		if f.Attribute == attribute && len(f.Values) > 0 {
			return f.Values[0].Text
		}
	}
	return ""
}
