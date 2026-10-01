package migrations

import (
	"context"
	"database/sql"

	"github.com/lithammer/shortuuid/v4"
	"github.com/pressly/goose/v3"
)

var FollowUps = goose.NewGoMigration(19, &goose.GoFunc{RunTx: followUps}, nil)

// followUpAttributes is the follow-ups schema as this migration created it;
// later changes belong in later migrations.
var followUpAttributes = []struct {
	slug, name, kind string
	multi            bool
	target           string
	options          []string
}{
	{slug: "name", name: "Action", kind: "text"},
	{slug: "status", name: "Status", kind: "status", options: []string{"Open", "Done", "Dismissed"}},
	{slug: "waiting_on", name: "Waiting on", kind: "select", options: []string{"Us", "Them"}},
	{slug: "review_on", name: "Review on", kind: "date"},
	{slug: "owner", name: "Owner", kind: "member"},
	{slug: "person", name: "Person", kind: "reference", target: "people"},
	{slug: "company", name: "Company", kind: "reference", target: "companies"},
	{slug: "deal", name: "Deal", kind: "reference", target: "deals"},
	{slug: "draft", name: "Draft", kind: "text"},
	{slug: "channel", name: "Channel", kind: "select", options: []string{"Email", "LinkedIn"}},
	{slug: "to", name: "To", kind: "email", multi: true},
	{slug: "cc", name: "Cc", kind: "email", multi: true},
	{slug: "draft_status", name: "Draft status", kind: "select", options: []string{"Draft", "Approved", "Sending", "Sent"}},
}

var followUpFilters = []struct{ name, filters string }{
	{"Needs attention", `[{"attribute":"status","operator":"is","value":"Open"},{"attribute":"review_on","operator":"on_or_before","value":"today"}]`},
	{"Waiting on them", `[{"attribute":"status","operator":"is","value":"Open"},{"attribute":"waiting_on","operator":"is","value":"Them"}]`},
}

// followUps gives every workspace without one the standard follow-ups object;
// a reference to an object the workspace lacks is left out.
func followUps(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
INSERT INTO objects (workspace_id, slug, name)
SELECT id, 'follow_ups', 'Follow-ups' FROM workspaces
WHERE NOT EXISTS (SELECT 1 FROM objects WHERE objects.workspace_id = workspaces.id AND objects.slug = 'follow_ups')
RETURNING id, workspace_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var created []struct{ id, workspace string }
	for rows.Next() {
		var o struct{ id, workspace string }
		if err := rows.Scan(&o.id, &o.workspace); err != nil {
			return err
		}
		created = append(created, o)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, o := range created {
		for _, a := range followUpAttributes {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO attributes (object_id, slug, name, type, multi, options, target_object_id)
SELECT $1, $2, $3, $4, $5, $6, target.id
FROM (SELECT (SELECT id FROM objects WHERE workspace_id = $7 AND slug = $8) AS id) target
WHERE $8 = '' OR target.id IS NOT NULL`,
				o.id, a.slug, a.name, a.kind, a.multi, append([]string{}, a.options...), o.workspace, a.target); err != nil {
				return err
			}
		}
		for _, f := range followUpFilters {
			if _, err := tx.ExecContext(ctx, `INSERT INTO saved_filters (id, workspace_id, object_id, name, filters) VALUES ($1, $2, $3, $4, $5)`,
				shortuuid.New(), o.workspace, o.id, f.name, f.filters); err != nil {
				return err
			}
		}
	}
	return nil
}
