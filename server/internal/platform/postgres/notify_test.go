package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// listening returns a function that waits for the next notification on
// channel, of a connection of pool's that LISTENs on it: its payload, or
// false when none comes within wait.
func listening(t *testing.T, pool *pgxpool.Pool, channel string) func(wait time.Duration) (string, bool) {
	t.Helper()
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Release)
	if _, err := conn.Exec(context.Background(), "LISTEN "+channel); err != nil {
		t.Fatal(err)
	}
	return func(wait time.Duration) (string, bool) {
		ctx, cancel := context.WithTimeout(context.Background(), wait)
		defer cancel()
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return "", false
		}
		return n.Payload, true
	}
}

// Notify sends when its transaction commits, in order, and not when it
// rolls back.
func TestNotifySendsWhenTheTransactionCommits(t *testing.T) {
	pool := newNotes(t, 4)
	tm := postgres.NewTxManager(pool, commitTimeout)
	next := listening(t, pool, "things")

	_ = tm.WithinTx(context.Background(), func(ctx context.Context) error {
		if err := postgres.Notify(ctx, "things", "lost"); err != nil {
			t.Fatal(err)
		}
		return errors.New("roll back")
	})
	err := tm.WithinTx(context.Background(), func(ctx context.Context) error {
		if err := postgres.Notify(ctx, "things", "one"); err != nil {
			return err
		}
		return postgres.Notify(ctx, "things", "two")
	})
	if err != nil {
		t.Fatal(err)
	}

	first, ok1 := next(5 * time.Second)
	second, ok2 := next(5 * time.Second)
	if !ok1 || !ok2 || first != "one" || second != "two" {
		t.Errorf("notifications = %q %v, %q %v; want one, two, and not the rolled back one", first, ok1, second, ok2)
	}
	if extra, ok := next(100 * time.Millisecond); ok {
		t.Errorf("another notification %q, want none", extra)
	}
}

// Outside a transaction Notify sends nothing and says so; a payload of
// 7999 bytes goes, one of 8000 does not.
func TestNotifyRefusesOutsideATransactionAndTooLong(t *testing.T) {
	pool := newNotes(t, 4)
	tm := postgres.NewTxManager(pool, commitTimeout)
	next := listening(t, pool, "things")

	if err := postgres.Notify(context.Background(), "things", "out"); !errors.Is(err, postgres.ErrNotifyOutsideTx) {
		t.Errorf("Notify() outside a transaction = %v, want ErrNotifyOutsideTx", err)
	}
	err := tm.WithinTx(context.Background(), func(ctx context.Context) error {
		if err := postgres.Notify(ctx, "things", strings.Repeat("a", postgres.MaxNotifyPayload+1)); !errors.Is(err, postgres.ErrNotifyTooLong) {
			t.Errorf("Notify() of 8000 bytes = %v, want ErrNotifyTooLong", err)
		}
		return postgres.Notify(ctx, "things", strings.Repeat("b", postgres.MaxNotifyPayload))
	})
	if got, ok := next(5 * time.Second); err != nil || !ok || len(got) != postgres.MaxNotifyPayload {
		t.Errorf("WithinTx() = %v; the notification of %d bytes came %v, want the 7999 bytes alone", err, len(got), ok)
	}
}
