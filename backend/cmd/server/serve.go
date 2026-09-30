package main

import (
	"context"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/app"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

func runServe(args []string) error {
	cfg, err := app.ParseConfig(args)
	if err != nil {
		return err
	}
	fxApp := fx.New(
		fx.StopTimeout(15*time.Second),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
		app.Options(cfg),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := fxApp.Start(ctx); err != nil {
		return err
	}
	<-fxApp.Wait()
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return fxApp.Stop(ctx)
}
