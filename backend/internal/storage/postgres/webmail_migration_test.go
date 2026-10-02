package postgres_test

import (
	"context"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
)

// A webmail skip rule is forgotten, and the senders it skipped on arrival
// return to triage; earlier decisions and work domain rules stand.
func TestWebmailRulesMigration(t *testing.T) {
	ctx := context.Background()
	db, dsn := legacy(t, 23)
	var workspace string
	if err := db.QueryRowContext(ctx, "INSERT INTO workspaces(name) VALUES('CAS') RETURNING id").Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	for _, seed := range []string{
		"INSERT INTO domain_rules(workspace_id,domain,triage,created_at) VALUES($1,'gmail.com','skipped',now() - interval '2 days')",
		"INSERT INTO domain_rules(workspace_id,domain,triage,created_at) VALUES($1,'supplier.com','skipped',now() - interval '2 days')",
		"INSERT INTO handles(workspace_id,kind,value,triage,decided_by,created_at) VALUES($1,'email','new@gmail.com','skipped','user',now() - interval '1 day')",
		"INSERT INTO handles(workspace_id,kind,value,triage,decided_by,created_at) VALUES($1,'email','old@gmail.com','skipped','user',now() - interval '3 days')",
		"INSERT INTO handles(workspace_id,kind,value,triage,decided_by,reason,created_at) VALUES($1,'email','spam@gmail.com','skipped','user','spam',now() - interval '1 day')",
		"INSERT INTO handles(workspace_id,kind,value,triage,decided_by,created_at) VALUES($1,'email','vendor@supplier.com','skipped','user',now() - interval '1 day')",
	} {
		if _, err := db.ExecContext(ctx, seed, workspace); err != nil {
			t.Fatal(err)
		}
	}
	store, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	triage := map[string]string{}
	rows, err := db.QueryContext(ctx, "SELECT value, triage FROM handles")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var value, verdict string
		if err := rows.Scan(&value, &verdict); err != nil {
			t.Fatal(err)
		}
		triage[value] = verdict
	}
	want := map[string]string{"new@gmail.com": "pending", "old@gmail.com": "skipped", "spam@gmail.com": "skipped", "vendor@supplier.com": "skipped"}
	for value, verdict := range want {
		if triage[value] != verdict {
			t.Errorf("%s is %s, want %s", value, triage[value], verdict)
		}
	}
	var rules []string
	rows, err = db.QueryContext(ctx, "SELECT domain FROM domain_rules ORDER BY domain")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			t.Fatal(err)
		}
		rules = append(rules, domain)
	}
	if len(rules) != 1 || rules[0] != "supplier.com" {
		t.Fatalf("rules after migration: %v", rules)
	}
}
