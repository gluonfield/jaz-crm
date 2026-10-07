package migrations

import (
	"context"
	"database/sql"

	"github.com/lithammer/shortuuid/v4"
	"github.com/pressly/goose/v3"
)

var PageIcons = goose.NewGoMigration(43, &goose.GoFunc{RunTx: pageIcons}, nil)

func pageIcons(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM objects WHERE slug = 'pages'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `INSERT INTO attributes (id, object_id, slug, name, type)
VALUES ($1, $2, 'icon', 'Icon', 'text') ON CONFLICT (object_id, slug) DO NOTHING`, shortuuid.New(), id); err != nil {
			return err
		}
	}
	return nil
}
