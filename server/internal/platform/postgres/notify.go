package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// MaxNotifyPayload is the longest payload NOTIFY takes, in bytes:
// PostgreSQL's limit is 8000, the terminating zero included.
const MaxNotifyPayload = 7999

// The errors of Notify.
var (
	ErrNotifyOutsideTx = errors.New("postgres: NOTIFY outside a transaction")
	ErrNotifyTooLong   = errors.New("postgres: NOTIFY payload too long")
)

// Notify sends payload on channel when the transaction in ctx commits (M5
// design 4.10): pg_notify runs in that transaction, so a write rolled back
// sends nothing and a listener hears of a write only once others can read
// it. Outside a transaction it is ErrNotifyOutsideTx, as NOTIFY there would
// go out at once, before the write it tells of commits; a payload longer
// than MaxNotifyPayload is ErrNotifyTooLong, which the caller shortens
// first. A transaction may send several. PostgreSQL folds those with the
// same channel and payload into one, so a caller whose notifications must
// all arrive makes their payloads differ.
func Notify(ctx context.Context, channel, payload string) error {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	if !ok {
		return ErrNotifyOutsideTx
	}
	if len(payload) > MaxNotifyPayload {
		return fmt.Errorf("%w: %d bytes on %s", ErrNotifyTooLong, len(payload), channel)
	}
	if _, err := tx.Exec(ctx, "SELECT pg_notify($1, $2)", channel, payload); err != nil {
		return fmt.Errorf("postgres: notify %s: %w", channel, err)
	}
	return nil
}
