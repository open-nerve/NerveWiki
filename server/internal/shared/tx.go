package shared

import "context"

// TxManager runs fn in one database transaction. The transaction travels in
// the context fn receives: the repositories of every module run their
// statements in it, and a nested WithinTx joins it. fn's error rolls the
// transaction back and is returned; otherwise it commits. platform/postgres
// implements it; bootstrap asserts that at compile time.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
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
