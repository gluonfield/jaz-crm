package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
)

var LoggedConversations = goose.NewGoMigration(49, &goose.GoFunc{RunTx: loggedConversations}, nil)

// loggedConversations keys logged messages as logging now does, merges logged
// conversations that share an original link, and makes that link their
// identity, so logging the thread again adds to it.
func loggedConversations(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT parts.id, parts.direction, parts.at, coalesce(parts.content, '') FROM parts
JOIN interactions ON interactions.id = parts.interaction_id
WHERE interactions.connection_id IS NULL AND parts.kind = 'message' ORDER BY parts.id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	keys := map[int64]string{}
	var order []int64
	for rows.Next() {
		var id int64
		var direction, content string
		var at time.Time
		if err := rows.Scan(&id, &direction, &at, &content); err != nil {
			return err
		}
		keys[id] = loggedMessageKey(direction, at, content)
		order = append(order, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range order {
		result, err := tx.ExecContext(ctx, `UPDATE parts SET external_id = $2, position = 0 WHERE id = $1
AND NOT EXISTS (SELECT 1 FROM parts other WHERE other.interaction_id = parts.interaction_id AND other.external_id = $2)`, id, keys[id])
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			if _, err := tx.ExecContext(ctx, `DELETE FROM parts WHERE id = $1`, id); err != nil {
				return err
			}
		}
	}
	for _, statement := range []string{
		`CREATE TEMP TABLE merges ON COMMIT DROP AS
SELECT id, first_value(id) OVER (PARTITION BY workspace_id, channel, url ORDER BY started_at, id) AS keeper
FROM interactions WHERE connection_id IS NULL AND url <> '' AND NOT skipped`,
		`DELETE FROM parts USING merges, parts kept
WHERE parts.interaction_id = merges.id AND merges.id <> merges.keeper AND kept.interaction_id = merges.keeper AND kept.external_id = parts.external_id`,
		`UPDATE parts SET interaction_id = merges.keeper FROM merges WHERE parts.interaction_id = merges.id AND merges.id <> merges.keeper`,
		`INSERT INTO links (interaction_id, record_id, source, created_at)
SELECT merges.keeper, links.record_id, links.source, links.created_at FROM links JOIN merges ON merges.id = links.interaction_id AND merges.id <> merges.keeper
ON CONFLICT DO NOTHING`,
		`INSERT INTO participants (interaction_id, handle_id, role)
SELECT merges.keeper, participants.handle_id, participants.role FROM participants JOIN merges ON merges.id = participants.interaction_id AND merges.id <> merges.keeper
ON CONFLICT DO NOTHING`,
		`DELETE FROM interactions USING merges WHERE interactions.id = merges.id AND merges.id <> merges.keeper`,
		`UPDATE interactions SET started_at = span.first_at, date_only = span.first_date_only,
  ended_at = CASE WHEN span.messages > 1 THEN span.last_at END
FROM (
  SELECT interaction_id, min(at) AS first_at, max(at) AS last_at, count(*) AS messages,
    (array_agg(date_only ORDER BY at, position, id))[1] AS first_date_only
  FROM parts WHERE kind = 'message' GROUP BY interaction_id
) AS span
WHERE interactions.id = span.interaction_id AND interactions.connection_id IS NULL`,
		`UPDATE interactions SET external_id = url FROM merges
WHERE interactions.id = merges.keeper AND NOT EXISTS (
  SELECT 1 FROM interactions other WHERE other.workspace_id = interactions.workspace_id AND other.channel = interactions.channel AND other.external_id = interactions.url)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

// loggedMessageKey is how logging keys a message as of this migration: by
// its side, day and opening words.
func loggedMessageKey(direction string, at time.Time, text string) string {
	opening := []rune(strings.Join(strings.Fields(text), " "))
	sum := sha256.Sum256([]byte(direction + "\n" + at.UTC().Format(time.DateOnly) + "\n" + string(opening[:min(len(opening), 32)])))
	return "message:" + hex.EncodeToString(sum[:16])
}
