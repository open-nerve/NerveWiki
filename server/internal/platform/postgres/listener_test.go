package postgres_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// heard records what a Listener tells, in order, on a channel.
type heard chan string

func (h heard) options() postgres.ListenerOptions {
	return postgres.ListenerOptions{
		OnNotify: func(p string) { h <- "notify " + p },
		OnListening: func(on bool) {
			if on {
				h <- "listening"
			} else {
				h <- "not listening"
			}
		},
		OnReconnect: func() { h <- "reconnect" },
		MinBackoff:  10 * time.Millisecond,
		MaxBackoff:  20 * time.Millisecond,
	}
}

// expect fails t unless the next things heard are want, each within 5 s.
func (h heard) expect(t *testing.T, want ...string) {
	t.Helper()
	for _, w := range want {
		select {
		case got := <-h:
			if got != w {
				t.Fatalf("heard %q, want %q", got, w)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("heard nothing, want %q", w)
		}
	}
}

// notify sends payload on channel in a transaction of its own.
func notify(t *testing.T, pool *pgxpool.Pool, channel, payload string) {
	t.Helper()
	err := postgres.NewTxManager(pool, commitTimeout).WithinTx(context.Background(), func(ctx context.Context) error {
		return postgres.Notify(ctx, channel, payload)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// listen runs a Listener on channel until the test ends, and returns what
// it heard and how to stop it, which waits for Run to return.
func listen(t *testing.T, pool *pgxpool.Pool, channel string) (heard, func()) {
	t.Helper()
	h := make(heard, 64)
	l := postgres.NewListener(pool, channel, slog.New(slog.DiscardHandler), h.options())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		l.Run(ctx)
	}()
	stop := func() {
		cancel()
		<-done
	}
	t.Cleanup(stop)
	return h, stop
}

// A Listener hands the notifications of its channel on, in order, on a
// connection it hijacked from the pool: the pool holds none of it.
func TestAListenerHearsItsChannel(t *testing.T) {
	pool := newNotes(t, 4)
	h, _ := listen(t, pool, "things")
	h.expect(t, "listening")

	notify(t, pool, "other", "not ours")
	notify(t, pool, "things", "one")
	notify(t, pool, "things", "two")

	h.expect(t, "notify one", "notify two")
	if s := pool.Stat(); s.AcquiredConns() != 0 {
		t.Errorf("the pool has %d connections acquired, want the listener's hijacked out of it", s.AcquiredConns())
	}
}

// A Listener whose connection is cut tells it, connects again after its
// backoff, tells that it listens and that it reconnected, and hears what
// comes then (M5 design 4.10).
func TestAListenerReconnects(t *testing.T) {
	pool := newNotes(t, 4)
	h, _ := listen(t, pool, "things")
	h.expect(t, "listening")

	if n := pgtest.TerminateListeners(t, pool, "things", 5*time.Second); n != 1 {
		t.Fatalf("terminated %d listeners, want 1", n)
	}
	h.expect(t, "not listening", "listening", "reconnect")
	notify(t, pool, "things", "after")
	h.expect(t, "notify after")
}

// Run returns when its context ends, with its connection closed: no
// backend listens on the channel any more.
func TestAListenerClosesItsConnectionWhenStopped(t *testing.T) {
	pool := newNotes(t, 4)
	h, stop := listen(t, pool, "things")
	h.expect(t, "listening")

	stop()

	h.expect(t, "not listening")
	var n int
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND query = 'LISTEN "things"'`).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 || time.Now().After(deadline) {
			break
		}
	}
	if n != 0 {
		t.Errorf("%d backends still listen 5 s after Run returned, want none", n)
	}
}
