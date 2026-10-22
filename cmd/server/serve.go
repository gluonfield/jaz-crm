package main

import "github.com/gluonfield/jaz-crm/internal/app"

func runServe(args []string) error {
	cfg, err := app.ParseConfig("serve", args)
	if err != nil {
		return err
	}
	return app.Run(app.Server(cfg))
}
