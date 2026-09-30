package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/gluonfield/jaz-crm/backend/internal/app"
)

func runAPIKey(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: apikey EMAIL [WORKSPACE] [flags]")
	}
	email, args := args[0], args[1:]
	var workspace string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		workspace, args = args[0], args[1:]
	}
	cfg, err := app.ParseConfig("apikey", args)
	if err != nil {
		return err
	}
	key, err := app.MintAPIKey(context.Background(), cfg, email, workspace)
	if err != nil {
		return err
	}
	fmt.Println(key)
	return nil
}
