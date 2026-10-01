package migrations

import (
	"context"
	"database/sql"

	"github.com/lithammer/shortuuid/v4"
	"github.com/pressly/goose/v3"
)

var DealFollowups = goose.NewGoMigration(15, &goose.GoFunc{RunTx: dealFollowups}, nil)

func dealFollowups(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
UPDATE attributes SET options = options[1:coalesce(array_position(options, 'Won'), cardinality(options) + 1) - 1]
  || ARRAY['On hold'] || options[coalesce(array_position(options, 'Won'), cardinality(options) + 1):cardinality(options)]
FROM objects WHERE attributes.object_id = objects.id AND objects.slug = 'deals'
  AND attributes.slug = 'stage' AND attributes.type = 'status' AND NOT 'On hold' = ANY(options);
INSERT INTO attributes (object_id, slug, name, type)
SELECT id, 'next_follow_up_date', 'Next follow-up date', 'date' FROM objects WHERE slug = 'deals'
ON CONFLICT (object_id, slug) DO NOTHING;
INSERT INTO attributes (object_id, slug, name, type)
SELECT id, 'next_action', 'Next action', 'text' FROM objects WHERE slug = 'deals'
ON CONFLICT (object_id, slug) DO NOTHING;
`); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, workspace_id FROM objects WHERE slug = 'deals'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var deals []struct{ id, workspace string }
	for rows.Next() {
		var deal struct{ id, workspace string }
		if err := rows.Scan(&deal.id, &deal.workspace); err != nil {
			return err
		}
		deals = append(deals, deal)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, deal := range deals {
		if _, err := tx.ExecContext(ctx, `INSERT INTO saved_filters (id, workspace_id, object_id, name, filters)
VALUES ($1, $2, $3, 'Due follow-ups', '[{"attribute":"next_follow_up_date","operator":"on_or_before","value":"today"},{"attribute":"stage","operator":"is_not","value":"Won"},{"attribute":"stage","operator":"is_not","value":"Lost"}]')
ON CONFLICT (workspace_id, object_id, lower(name)) DO NOTHING`, shortuuid.New(), deal.workspace, deal.id); err != nil {
			return err
		}
	}
	return nil
}
