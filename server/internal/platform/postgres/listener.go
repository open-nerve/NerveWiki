package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ListenerOptions are what a Listener tells and how long it waits between
// connections.
type ListenerOptions struct {
	// OnNotify gets each notification's payload, in the order they come, on
	// the Listener's goroutine: it must not block.
	OnNotify func(payload string)
	// OnListening tells whether LISTEN is in effect: true once it is on a
	// connection, false once that connection is lost or closed.
	OnListening func(listening bool)
	// OnReconnect comes after OnListening(true) on every connection but the
	// first: what was sent while there was none is lost.
	OnReconnect func()
	// The wait before connecting again, doubled after each failure from
	// MinBackoff (100 ms when zero) up to MaxBackoff (5 s when zero).
	MinBackoff, MaxBackoff time.Duration
}

// Listener holds one connection that LISTENs on a channel (M5 design 4.10).
// It takes the connection from the pool and hijacks it, as River does, so
// that it does not hold one of database.max_conns; the connection is the
// Listener's alone, and closed when Run returns.
type Listener struct {
	pool    *pgxpool.Pool
	channel string
	logger  *slog.Logger
	opts    ListenerOptions
}

// NewListener returns a Listener on channel; it connects when Run starts.
func NewListener(pool *pgxpool.Pool, channel string, logger *slog.Logger, opts ListenerOptions) *Listener {
	if opts.MinBackoff <= 0 {
		opts.MinBackoff = 100 * time.Millisecond
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = 5 * time.Second
	}
	return &Listener{pool: pool, channel: channel, logger: logger, opts: opts}
}

// Run listens until ctx ends, then returns with its connection closed. A
// connection that fails is closed and replaced after the backoff; it
// tells OnListening(false), and the next one OnListening(true) and
// OnReconnect.
func (l *Listener) Run(ctx context.Context) {
	backoff, listened := l.opts.MinBackoff, false
	for {
		conn, err := l.connect(ctx)
		if err == nil {
			backoff = l.opts.MinBackoff
			l.logger.InfoContext(ctx, "notification listener listening", slog.String("channel", l.channel), slog.Bool("again", listened))
			l.opts.OnListening(true)
			if listened {
				l.opts.OnReconnect()
			}
			listened = true
			err = l.listen(ctx, conn)
			l.opts.OnListening(false)
		}
		if ctx.Err() != nil {
			return
		}
		l.logger.WarnContext(ctx, "notification listener lost its connection",
			slog.String("channel", l.channel), slog.Duration("retry_in", backoff), slog.Any("error", err))
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, l.opts.MaxBackoff)
	}
}

// connect hijacks a connection of the pool and LISTENs on it.
func (l *Listener) connect(ctx context.Context) (*pgx.Conn, error) {
	pooled, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: listener: acquire: %w", err)
	}
	conn := pooled.Hijack()
	if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{l.channel}.Sanitize()); err != nil {
		closeConn(conn)
		return nil, fmt.Errorf("postgres: listener: LISTEN %s: %w", l.channel, err)
	}
	return conn, nil
}

// listen hands every notification on conn to OnNotify until conn fails or
// ctx ends, then closes conn.
func (l *Listener) listen(ctx context.Context, conn *pgx.Conn) error {
	defer closeConn(conn)
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("postgres: listener: %w", err)
		}
		l.opts.OnNotify(n.Payload)
	}
}

// closeConn closes conn, waiting a little for the server to hear of it
// even when the caller's context has ended.
func closeConn(conn *pgx.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = conn.Close(ctx)
}
