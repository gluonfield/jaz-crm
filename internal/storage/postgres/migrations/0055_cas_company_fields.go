package migrations

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/lithammer/shortuuid/v4"
	"github.com/pressly/goose/v3"
)

var CASCompanyFields = goose.NewGoMigration(55, &goose.GoFunc{RunTx: casCompanyFields}, nil)

// These are CAS-owned custom properties. Other workspaces keep their schemas.
const casWorkspace = "cf52341c-db12-46a3-a60e-4060c0380200"

func casCompanyFields(ctx context.Context, tx *sql.Tx) error {
	var object, description, categories string
	err := tx.QueryRowContext(ctx, "SELECT id FROM objects WHERE workspace_id = $1 AND slug = 'companies'", casWorkspace).Scan(&object)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, "SELECT id FROM attributes WHERE object_id = $1 AND slug = 'description' AND type = 'text' AND NOT multi AND NOT archived", object).Scan(&description); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, "SELECT id FROM attributes WHERE object_id = $1 AND slug = 'categories' AND type = 'select' AND multi AND NOT archived", object).Scan(&categories); err != nil {
		return err
	}
	// Relocating existing information must not make every company look newly edited.
	if _, err := tx.ExecContext(ctx, "ALTER TABLE record_values DISABLE TRIGGER record_value_updated"); err != nil {
		return err
	}
	type value struct {
		id    int64
		text  string
		at    time.Time
		actor sql.NullString
	}
	type company struct {
		id     string
		fields map[string][]value
	}
	var companies []company
	rows, err := tx.QueryContext(ctx, "SELECT r.id, a.slug, v.id, v.text, v.active_from, v.actor_id FROM records r JOIN record_values v ON v.record_id = r.id AND v.active_until IS NULL JOIN attributes a ON a.id = v.attribute_id WHERE r.object_id = $1 AND a.slug IN ('description', 'primary_activities', 'key_products', 'categories', 'manufacturer', 'outreach_notes') AND NOT a.archived ORDER BY r.id, a.slug, v.id", object)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, slug string
		var v value
		if err := rows.Scan(&id, &slug, &v.id, &v.text, &v.at, &v.actor); err != nil {
			rows.Close()
			return err
		}
		if len(companies) == 0 || companies[len(companies)-1].id != id {
			companies = append(companies, company{id: id, fields: map[string][]value{}})
		}
		fields := companies[len(companies)-1].fields
		fields[slug] = append(fields[slug], v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	normalize := func(text string) string {
		return strings.ToLower(strings.Join(strings.Fields(text), " "))
	}
	boilerplate := regexp.MustCompile("^Leadership (?:re-)?search completed — [0-9]+ contacts?(?:\\(s\\))? found via Exa on [0-9]{4}-[0-9]{2}-[0-9]{2}$")
	for _, c := range companies {
		var original, merged string
		if current := c.fields["description"]; len(current) != 0 {
			original = current[0].text
		}
		merged = original
		for _, slug := range []string{"primary_activities", "key_products"} {
			for _, v := range c.fields[slug] {
				if text := strings.TrimSpace(v.text); text != "" && !slices.ContainsFunc(strings.Split(merged, "\n\n"), func(paragraph string) bool { return normalize(paragraph) == normalize(text) }) {
					if strings.TrimSpace(merged) != "" {
						merged += "\n\n"
					}
					merged += text
				}
			}
		}
		if merged != original {
			if _, err := tx.ExecContext(ctx, "UPDATE record_values SET active_until = now() WHERE record_id = $1 AND attribute_id = $2 AND active_until IS NULL", c.id, description); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO record_values(record_id, attribute_id, text, source) VALUES ($1, $2, $3, 'agent')", c.id, description, merged); err != nil {
				return err
			}
		}
		manufacturer := slices.ContainsFunc(c.fields["manufacturer"], func(v value) bool { return strings.EqualFold(strings.TrimSpace(v.text), "Yes") })
		tagged := slices.ContainsFunc(c.fields["categories"], func(v value) bool { return strings.EqualFold(v.text, "Manufacturer") })
		if manufacturer && !tagged {
			if _, err := tx.ExecContext(ctx, "INSERT INTO record_values(record_id, attribute_id, text, source) VALUES ($1, $2, 'Manufacturer', 'agent')", c.id, categories); err != nil {
				return err
			}
		}
		for _, v := range c.fields["outreach_notes"] {
			lines := strings.Split(strings.ReplaceAll(v.text, "<br>", "\n"), "\n")
			onlyBookkeeping := true
			for _, line := range lines {
				if line = strings.TrimSpace(line); line != "" && !boilerplate.MatchString(line) {
					onlyBookkeeping = false
				}
			}
			// Import bookkeeping remains recoverable in the archived property.
			if onlyBookkeeping {
				continue
			}
			id := shortuuid.New()
			if _, err := tx.ExecContext(ctx, "INSERT INTO interactions(id, workspace_id, user_id, kind, external_id, title, started_at) VALUES ($1, $2, $3, 'note', $4, 'Outreach Notes', $5)", id, casWorkspace, v.actor, "company-outreach-notes:"+c.id, v.at); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO parts(interaction_id, kind, external_id, author_name, at, content) VALUES ($1, 'note', 'note', 'Imported', $2, $3)", id, v.at, v.text); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO links(interaction_id, record_id, source) VALUES ($1, $2, 'agent')", id, c.id); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE attributes SET options = array_append(options, 'Manufacturer') WHERE id = $1 AND NOT ('Manufacturer' = ANY(options))", categories); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE attributes SET archived = true WHERE object_id = $1 AND slug IN ('outreach_notes', 'primary_activities', 'key_products', 'manufacturer', 'fit_profile', 'last_activity_date')", object); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE record_values ENABLE TRIGGER record_value_updated")
	return err
}
