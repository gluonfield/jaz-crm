package migrations

import (
	"context"
	"database/sql"

	"github.com/lithammer/shortuuid/v4"
	"github.com/pressly/goose/v3"
)

var PrivateFollowUpFilters = goose.NewGoMigration(61, &goose.GoFunc{RunTx: privateFollowUpFilters}, nil)

func privateFollowUpFilters(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT f.id, u.id FROM saved_filters f
JOIN objects o ON o.id = f.object_id AND o.slug = 'follow_ups'
JOIN users u ON u.workspace_id = f.workspace_id
WHERE f.user_id IS NULL ORDER BY f.id, u.created_at, u.id`)
	if err != nil {
		return err
	}
	var copies []struct{ filter, user string }
	for rows.Next() {
		var copy struct{ filter, user string }
		if err := rows.Scan(&copy.filter, &copy.user); err != nil {
			rows.Close()
			return err
		}
		copies = append(copies, copy)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	previous := ""
	for _, copy := range copies {
		if copy.filter != previous {
			_, err = tx.ExecContext(ctx, `UPDATE saved_filters SET user_id = $1,
name = CASE WHEN lower(name) IN ('everyone', 'assigned to me') THEN name || ' (' || id || ')' ELSE name END WHERE id = $2`, copy.user, copy.filter)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO saved_filters (id, workspace_id, object_id, name, query, filters, user_id)
SELECT $1, workspace_id, object_id, name, query, filters, $2 FROM saved_filters WHERE id = $3`, shortuuid.New(), copy.user, copy.filter)
		}
		if err != nil {
			return err
		}
		previous = copy.filter
	}
	return nil
}
