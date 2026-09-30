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
