package db

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Listen calls handle with each payload sent on a Postgres NOTIFY channel until
// ctx ends, reconnecting after errors. Notifications sent while it reconnects
// are lost, so callers must not depend on every one arriving.
func Listen(ctx context.Context, pool *pgxpool.Pool, channel string, handle func(payload string)) {
	for ctx.Err() == nil {
		if err := listenOnce(ctx, pool, channel, handle); err != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "listen failed, retrying", "channel", channel, "err", err)
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
			}
		}
	}
}

func listenOnce(ctx context.Context, pool *pgxpool.Pool, channel string, handle func(string)) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	// The channel name is a constant from our code, never user input.
	if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
		return err
	}
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		handle(n.Payload)
	}
}
