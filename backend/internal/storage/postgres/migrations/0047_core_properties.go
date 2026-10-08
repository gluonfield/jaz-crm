package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lithammer/shortuuid/v4"
	"github.com/pressly/goose/v3"
)

var CoreProperties = goose.NewGoMigration(47, &goose.GoFunc{RunTx: coreProperties}, nil)

// This snapshot stays fixed so future catalog edits cannot alter old migrations.
var corePropertyDefinitions = []struct{ object, slug, name, kind string }{
	{"companies", "website", "Website", "url"},
	{"companies", "linkedin_url", "LinkedIn", "url"},
	{"companies", "industry", "Industry", "select"},
	{"companies", "hq_city", "Headquarters city", "text"},
	{"companies", "hq_state", "Headquarters state / region", "text"},
	{"companies", "hq_country", "Headquarters country", "select"},
	{"companies", "employee_count", "Employee count", "number"},
	{"companies", "owner", "Owner", "member"},
	{"companies", "notes", "Notes", "text"},
	{"people", "linkedin_url", "LinkedIn", "url"},
	{"people", "owner", "Owner", "member"},
	{"people", "notes", "Notes", "text"},
	{"deals", "expected_close_date", "Expected close date", "date"},
}

func coreProperties(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, slug FROM objects WHERE slug IN ('companies', 'people', 'deals') ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var objects []struct{ id, slug string }
	for rows.Next() {
		var object struct{ id, slug string }
		if err := rows.Scan(&object.id, &object.slug); err != nil {
			return err
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, object := range objects {
		for _, property := range corePropertyDefinitions {
			if property.object != object.slug {
				continue
			}
			result, err := tx.ExecContext(ctx, `INSERT INTO attributes (id, object_id, slug, name, type)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (object_id, slug) DO UPDATE SET archived = false
WHERE attributes.type = EXCLUDED.type AND NOT attributes.multi AND attributes.target_object_id IS NULL`,
				shortuuid.New(), object.id, property.slug, property.name, property.kind)
			if err != nil {
				return err
			}
			n, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if n == 0 {
				return fmt.Errorf("core property %s.%s on object %s has an incompatible definition", object.slug, property.slug, object.id)
			}
		}
	}
	return nil
}
