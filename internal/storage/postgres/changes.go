package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Listen follows the changes channel the migrations' triggers notify, on a
// connection of its own: a listening connection cannot go back to the pool.
func (s *Store) Listen(ctx context.Context, ready func(), changed func(workspaceID string)) error {
	conn, err := pgx.ConnectConfig(ctx, s.pool.Config().ConnConfig)
	if err != nil {
		return err
	}
	defer conn.Close(context.WithoutCancel(ctx))
	if _, err := conn.Exec(ctx, "LISTEN changes"); err != nil {
		return err
	}
	ready()
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		changed(n.Payload)
	}
}
