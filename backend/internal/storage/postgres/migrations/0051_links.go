package migrations

import (
	"context"
	"database/sql"
	"net/url"
	"strings"

	"github.com/pressly/goose/v3"
)

var Links = goose.NewGoMigration(51, &goose.GoFunc{RunTx: links}, nil)

// links turns the LinkedIn field of people and companies into Links, many
// links that identify their record, keeping its values and history. People's
// X values join them and the X field goes; profile handles become link
// handles. Moving values changes no record, so their last update stays.
func links(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`ALTER TABLE record_values DISABLE TRIGGER record_value_updated`,
		`UPDATE attributes SET slug = 'links', name = 'Links', multi = true, is_unique = true
FROM objects WHERE objects.id = attributes.object_id AND objects.slug IN ('people', 'companies') AND attributes.slug = 'linkedin_url'`,
		`UPDATE record_values SET attribute_id = links.id FROM attributes x, attributes links
WHERE record_values.attribute_id = x.id AND x.slug = 'x_url' AND links.object_id = x.object_id AND links.slug = 'links'`,
		`DELETE FROM attributes USING objects WHERE objects.id = attributes.object_id AND objects.slug = 'people' AND attributes.slug = 'x_url'`,
		`UPDATE saved_filters SET filters = (
  SELECT jsonb_agg(CASE WHEN f->>'attribute' IN ('linkedin_url', 'x_url') THEN jsonb_set(f, '{attribute}', '"links"') ELSE f END ORDER BY n)
  FROM jsonb_array_elements(saved_filters.filters) WITH ORDINALITY AS e(f, n))
WHERE jsonb_path_exists(filters, '$[*] ? (@.attribute == "linkedin_url" || @.attribute == "x_url")')`,
		`ALTER TABLE handles DROP CONSTRAINT handles_kind_check`,
		`UPDATE handles SET kind = 'link' WHERE kind IN ('linkedin', 'x', 'telegram')`,
		`ALTER TABLE handles ADD CONSTRAINT handles_kind_check CHECK (kind IN ('email', 'phone', 'link'))`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT record_values.id, record_values.text FROM record_values
JOIN attributes ON attributes.id = record_values.attribute_id AND attributes.slug = 'links'
JOIN objects ON objects.id = attributes.object_id AND objects.slug IN ('people', 'companies')
WHERE record_values.text IS NOT NULL ORDER BY record_values.id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	keys := map[int64]string{}
	var order []int64
	for rows.Next() {
		var id int64
		var text string
		if err := rows.Scan(&id, &text); err != nil {
			return err
		}
		keys[id] = linkKey(text)
		order = append(order, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range order {
		if _, err := tx.ExecContext(ctx, `UPDATE record_values SET unique_key = $2 WHERE id = $1`, id, keys[id]); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `ALTER TABLE record_values ENABLE TRIGGER record_value_updated`)
	return err
}

// linkKey is how links compare as of this migration: host and path in lower
// case, without scheme, www, query, fragment or trailing slash, with
// LinkedIn's country hosts and twitter.com folded into linkedin.com and x.com.
func linkKey(raw string) string {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.Contains(u.Hostname(), ".") {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	switch {
	case strings.HasSuffix(host, ".linkedin.com"):
		host = "linkedin.com"
	case host == "twitter.com" || host == "mobile.twitter.com":
		host = "x.com"
	}
	return host + strings.ToLower(strings.TrimRight(u.Path, "/"))
}
