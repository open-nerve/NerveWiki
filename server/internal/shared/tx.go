package shared

import (
	"context"
	"errors"
)

// TxManager runs fn in one database transaction. The transaction travels in
// the context fn receives: the repositories of every module run their
// statements in it, and a nested WithinTx joins it. fn's error rolls the
// transaction back and is returned; otherwise it commits. platform/postgres
// implements it; bootstrap asserts that at compile time.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// TxNotBegun reports whether err is a transaction's that could not begin,
// none of fn's statements run: its connection is broken, or the pool
// closed. platform/postgres's TxManager marks such an error by a
// TxNotBegun method, declared here by structure. Its cause is the
// connection's, not anything fn wrote (M8/P5A fix check M-1).
func TxNotBegun(err error) bool {
	var e interface{ TxNotBegun() bool }
	return errors.As(err, &e) && e.TxNotBegun()
}

// Snapshots runs fn in a read-only transaction that sees the database as of
// one moment, whatever commits meanwhile (M7/P5 design 3.3): an export reads
// a notebook's tree, contents and links as they were together. The
// repositories find it in the context, as they find WithinTx's. It must be
// the outermost transaction, and holds none: a WithinTx in it fails.
// platform/postgres implements it; bootstrap asserts that at compile time.
type Snapshots interface {
	WithinSnapshot(ctx context.Context, fn func(ctx context.Context) error) error
}
