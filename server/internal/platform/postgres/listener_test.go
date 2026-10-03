package postgres_test

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"sync"
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
	return listenPinging(t, pool, channel, 0)
}

// listenPinging is listen with the Listener's PingInterval.
func listenPinging(t *testing.T, pool *pgxpool.Pool, channel string, ping time.Duration) (heard, func()) {
	t.Helper()
	h := make(heard, 64)
	opts := h.options()
	opts.PingInterval = ping
	l := postgres.NewListener(pool, channel, slog.New(slog.DiscardHandler), opts)
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

// A Listener's connection that stays quiet past PingInterval is pinged and
// kept: the notifications that come after several pings arrive on it.
func TestAListenerKeepsAQuietConnection(t *testing.T) {
	pool := newNotes(t, 4)
	h, _ := listenPinging(t, pool, "things", 250*time.Millisecond)
	h.expect(t, "listening")

	time.Sleep(time.Second)
	notify(t, pool, "things", "late")

	h.expect(t, "notify late")
}

// A Listener whose connection goes silent without a word of its end, as
// after a failover or a NAT that forgot it, finds it lost at its ping and
// connects again: OnReconnect tells that the notifications meanwhile were
// lost (M5 design 4.10).
func TestAListenerFindsASilentConnectionLost(t *testing.T) {
	notes := newNotes(t, 4)
	silent := newSilencer(t, notes.Config().ConnConfig.Host, notes.Config().ConnConfig.Port)
	cfg := notes.Config()
	cfg.ConnConfig.Host, cfg.ConnConfig.Port = "127.0.0.1", silent.port()
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	h, _ := listenPinging(t, pool, "things", 250*time.Millisecond)
	h.expect(t, "listening")

	silent.silence()

	h.expect(t, "not listening", "listening", "reconnect")
	notify(t, notes, "things", "after")
	h.expect(t, "notify after")
}

// silencer is a TCP proxy to a database server whose connections can be
// silenced: they stay open and pass nothing more, either way. A connection
// made after it silences passes as usual.
type silencer struct {
	ln     net.Listener
	mu     sync.Mutex
	open   []*silenced
	closed chan struct{}
}

// silenced is one proxied connection, quiet once its channel is closed.
type silenced struct{ quiet chan struct{} }

func newSilencer(t *testing.T, host string, port uint16) *silencer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &silencer{ln: ln, closed: make(chan struct{})}
	t.Cleanup(func() {
		close(s.closed)
		_ = ln.Close()
	})
	target := net.JoinHostPort(host, strconv.Itoa(int(port)))
	go func() {
		for {
			client, err := ln.Accept()
			if err != nil {
				return
			}
			server, err := net.Dial("tcp", target)
			if err != nil {
				_ = client.Close()
				continue
			}
			c := &silenced{quiet: make(chan struct{})}
			s.mu.Lock()
			s.open = append(s.open, c)
			s.mu.Unlock()
			go s.pass(c, client, server)
			go s.pass(c, server, client)
		}
	}()
	return s
}

func (s *silencer) port() uint16 { return uint16(s.ln.Addr().(*net.TCPAddr).Port) }

// silence quiets every connection open now.
func (s *silencer) silence() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.open {
		close(c.quiet)
	}
	s.open = nil
}

// pass copies from src to dst until either fails, or c is quieted: then it
// holds both open, passing nothing, until the test ends.
func (s *silencer) pass(c *silenced, dst, src net.Conn) {
	defer func() {
		_ = dst.Close()
		_ = src.Close()
	}()
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		select {
		case <-c.quiet:
			<-s.closed
			return
		default:
		}
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}
