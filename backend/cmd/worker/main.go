package main

import (
	"fmt"
	"os"

	"github.com/gluonfield/jaz-crm/backend/internal/app"
)

func main() {
	cfg, err := app.ParseConfig("worker", os.Args[1:])
	if err == nil {
		err = app.Run(app.Worker(cfg))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "worker: %v\n", err)
		os.Exit(1)
	}
}
