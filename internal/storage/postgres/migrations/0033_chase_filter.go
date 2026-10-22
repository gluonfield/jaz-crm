package migrations

import (
	"context"
	"database/sql"

	"github.com/lithammer/shortuuid/v4"
	"github.com/pressly/goose/v3"
)

var ChaseFilter = goose.NewGoMigration(33, &goose.GoFunc{RunTx: chaseFilter}, nil)

func chaseFilter(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, workspace_id FROM objects WHERE slug = 'follow_ups'`)
	if err != nil {
		return err
	}
	var objects []struct{ id, workspace string }
	for rows.Next() {
		var object struct{ id, workspace string }
		if err := rows.Scan(&object.id, &object.workspace); err != nil {
			rows.Close()
			return err
		}
		objects = append(objects, object)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, object := range objects {
		if _, err := tx.ExecContext(ctx, `WITH saved AS (INSERT INTO saved_filters (id, workspace_id, object_id, name, filters)
VALUES ($1, $2, $3, 'Chase', '[{"attribute":"status","operator":"is","value":"Open"},{"attribute":"waiting_on","operator":"is","value":"Them"},{"attribute":"action_date","operator":"on_or_before","value":"now"}]')
ON CONFLICT (workspace_id, object_id, lower(name)) DO UPDATE SET filters = EXCLUDED.filters
WHERE saved_filters.filters = '[{"attribute":"status","operator":"is","value":"Open"},{"attribute":"waiting_on","operator":"is","value":"Them"},{"attribute":"action_date","operator":"before","value":"today"}]'::jsonb
RETURNING id, filters)
UPDATE active_filters SET filters = saved.filters FROM saved
WHERE active_filters.saved_id = saved.id
  AND active_filters.filters = '[{"attribute":"status","operator":"is","value":"Open"},{"attribute":"waiting_on","operator":"is","value":"Them"},{"attribute":"action_date","operator":"before","value":"today"}]'::jsonb`, shortuuid.New(), object.workspace, object.id); err != nil {
			return err
		}
	}
	return nil
}
