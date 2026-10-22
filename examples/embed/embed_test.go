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
		`CREATE TABLE companies (id text PRIMARY KEY, name text NOT NULL)`,
		`CREATE TABLE users (id text PRIMARY KEY, name text NOT NULL, email text NOT NULL)`,
		`CREATE TABLE company_memberships (user_id text REFERENCES users, company_id text REFERENCES companies, PRIMARY KEY (user_id, company_id))`,
		`INSERT INTO companies VALUES ('co-acme', 'Acme'), ('co-rival', 'Rival')`,
		`INSERT INTO users VALUES ('u-ada', 'Ada', 'ada@acme.test'), ('u-bob', 'Bob', 'bob@acme.test'), ('u-eve', 'Eve', 'eve@rival.test')`,
		`INSERT INTO company_memberships VALUES ('u-ada', 'co-acme'), ('u-bob', 'co-acme'), ('u-bob', 'co-rival'), ('u-eve', 'co-rival')`,
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

	// signedIn is the host's request path: it admits a signed-in user to a
	// company they belong to and hands the CRM what its own tables say.
	signedIn := func(userID, companyID string) records.Actor {
		t.Helper()
		var u crm.User
		err := pool.QueryRow(ctx, `SELECT m.company_id, u.id, u.name, u.email FROM users u
			JOIN company_memberships m ON m.user_id = u.id WHERE u.id = $1 AND m.company_id = $2`, userID, companyID).Scan(&u.Tenant, &u.ID, &u.Name, &u.Email)
		if err != nil {
			t.Fatalf("%s is not a member of %s: %v", userID, companyID, err)
		}
		actor, err := c.Actor(ctx, u)
		if err != nil {
			t.Fatal(err)
		}
		return actor
	}

	ada := signedIn("u-ada", "co-acme")
	if ada.WorkspaceID != "co-acme" || ada.MemberID == "" || ada.MemberID == "u-ada" {
		t.Fatalf("ada acts as %+v", ada)
	}
	if again := signedIn("u-ada", "co-acme"); again != ada {
		t.Fatalf("ada's next request acts as %+v, was %+v", again, ada)
	}
	bob := signedIn("u-bob", "co-acme")
	if bob.WorkspaceID != ada.WorkspaceID || bob.MemberID == ada.MemberID {
		t.Fatalf("bob acts as %+v beside ada %+v", bob, ada)
	}

	company, _, err := c.Records.Upsert(ctx, ada, ada.Source(), records.Write{
		Object: "companies",
		Set:    map[string][]string{"name": {"Acme"}, "domains": {"acme.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	agent := bob
	agent.Agent = true
	same, skipped, err := c.Records.Upsert(ctx, agent, agent.Source(), records.Write{
		Object: "companies",
		Set:    map[string][]string{"domains": {"acme.com"}, "name": {"ACME Inc"}, "description": {"Presses"}},
	})
	if err != nil || same.ID != company.ID || len(skipped) != 1 || skipped[0].Attribute != "name" {
		t.Fatalf("agent write matched %s, skipped %+v: %v", same.ID, skipped, err)
	}
	if _, _, err := c.Records.Upsert(ctx, bob, bob.Source(), records.Write{Object: "companies", RecordID: company.ID, Set: map[string][]string{"name": {"Acme Corp"}}}); err != nil {
		t.Fatal(err)
	}
	read, err := c.Records.Get(ctx, ada, company.ID)
	if err != nil || text(read, "name") != "Acme Corp" || text(read, "description") != "Presses" {
		t.Fatalf("read %+v: %v", read.Fields, err)
	}

	exec(t, pool, `UPDATE users SET name = 'Robert', email = 'robert@acme.test' WHERE id = 'u-bob'`)
	if renamed := signedIn("u-bob", "co-acme"); renamed != bob {
		t.Fatalf("bob acts as %+v after the host renamed him, was %+v", renamed, bob)
	}
	history, err := c.Records.History(ctx, ada, company.ID)
	if err != nil {
		t.Fatal(err)
	}
	var names, agentEdits []string
	for _, change := range history {
		if change.Attribute == "name" && change.Source == records.SourceUser {
			names = append(names, change.Actor+": "+change.Value.Text)
		}
		if change.Source == records.SourceAgent && change.Actor == "Robert" {
			agentEdits = append(agentEdits, change.Attribute)
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"Ada: Acme", "Robert: Acme Corp"}) || !slices.Contains(agentEdits, "description") {
		t.Fatalf("history lost who wrote what: names %v, agent edits %v", names, agentEdits)
	}

	note, err := c.Interactions.Log(ctx, ada, interactions.Entry{Kind: "note", Text: "Wants a quote for 5,000 brackets", Records: []string{company.ID}})
	if err != nil {
		t.Fatal(err)
	}
	timeline, err := c.Interactions.Timeline(ctx, bob, company.ID, nil, "", false, 10)
	if err != nil || len(timeline) != 1 || timeline[0].ID != note.ID {
		t.Fatalf("timeline %+v: %v", timeline, err)
	}

	eve := signedIn("u-eve", "co-rival")
	bobAtRival := signedIn("u-bob", "co-rival")
	if eve.WorkspaceID != "co-rival" || bobAtRival.WorkspaceID != "co-rival" || bobAtRival.MemberID == bob.MemberID {
		t.Fatalf("rival actors %+v and %+v", eve, bobAtRival)
	}
	for _, rival := range []records.Actor{eve, bobAtRival} {
		if got, err := c.Records.Get(ctx, rival, company.ID); err == nil {
			t.Fatalf("rival read %s", got.ID)
		}
		if _, _, err := c.Records.Upsert(ctx, rival, rival.Source(), records.Write{Object: "companies", RecordID: company.ID, Set: map[string][]string{"name": {"Taken"}}}); err == nil {
			t.Fatal("rival wrote to another tenant's record")
		}
		if found, _, err := c.Records.Search(ctx, rival, records.Search{Object: "companies", Query: "acme"}); err != nil || len(found) != 0 {
			t.Fatalf("rival found %d companies: %v", len(found), err)
		}
		if _, err := c.Interactions.Log(ctx, rival, interactions.Entry{Kind: "note", Text: "Theirs", Records: []string{company.ID}}); err == nil {
			t.Fatal("rival linked a note to another tenant's record")
		}
	}
	theirs, _, err := c.Records.Upsert(ctx, eve, eve.Source(), records.Write{Object: "companies", Set: map[string][]string{"name": {"Acme"}, "domains": {"acme.com"}}})
	if err != nil || theirs.ID == company.ID {
		t.Fatalf("rival's acme.com matched %s: %v", theirs.ID, err)
	}

	mallory := crm.User{Tenant: "co-acme", ID: "u-mallory", Name: "Mallory", Email: "ADA@acme.test"}
	if got, err := c.Actor(ctx, mallory); !errors.As(err, new(errs.Invalid)) {
		t.Fatalf("a second user took ada's email and acts as %+v: %v", got, err)
	}
	if _, err := c.Actor(ctx, crm.User{ID: "u-ada", Email: "ada@acme.test"}); !errors.As(err, new(errs.Invalid)) {
		t.Fatalf("acted without a tenant: %v", err)
	}

	// Users entering a new tenant at once agree on one workspace and one
	// member each. Opening the connections first and repeating the race
	// keep a caller from finishing before the others start.
	standard, err := c.Records.Objects(ctx, ada, true)
	if err != nil {
		t.Fatal(err)
	}
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
		tenant := fmt.Sprint("co-new-", round)
		users := []crm.User{
			{Tenant: tenant, ID: "u-ada", Name: "Ada", Email: "ada@acme.test"},
			{Tenant: tenant, ID: "u-ada", Name: "Ada", Email: "ada@acme.test"},
			{Tenant: tenant, ID: "u-eve", Name: "Eve", Email: "eve@rival.test"},
			{Tenant: tenant, ID: "u-eve", Name: "Eve", Email: "eve@rival.test"},
		}
		actors := make([]records.Actor, len(users))
		failures := make([]error, len(users))
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range users {
			wg.Go(func() {
				<-start
				actors[i], failures[i] = c.Actor(ctx, users[i])
			})
		}
		close(start)
		wg.Wait()
		if errors.Join(failures...) != nil || actors[0] != actors[1] || actors[2] != actors[3] || actors[0].MemberID == actors[2].MemberID || actors[0].WorkspaceID != tenant {
			t.Fatalf("concurrent first uses of a tenant disagree: %+v %v", actors, failures)
		}
		objects, err := c.Records.Objects(ctx, actors[0], true)
		if err != nil || len(objects) != len(standard) {
			t.Fatalf("new tenant has %d objects, want %d: %v", len(objects), len(standard), err)
		}
	}

	if n := count(t, pool, `SELECT count(*) FROM users`); n != 3 {
		t.Fatalf("host users changed: %d", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'users'`); n != 3 {
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
