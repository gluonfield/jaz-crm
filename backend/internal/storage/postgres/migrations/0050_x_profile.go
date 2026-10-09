package migrations

import "github.com/pressly/goose/v3"

var XProfile = goose.NewGoMigration(50, &goose.GoFunc{RunTx: coreProperties([]coreProperty{{"people", "x_url", "X", "url"}})}, nil)
