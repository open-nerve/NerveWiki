package postgresadapter_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// Only the cross-module path inputs are supplied here. Index.PagesChanged,
// fresh domain resolution, Store SQL, its advisory lock and TxManager are real.
// Every node in this small fixture is at the root and has no children.
type setIndexPages []domain.Node

func (p setIndexPages) ByKeys(_ context.Context, _ uuid.UUID, keys []string) ([]domain.Node, error) {
	var out []domain.Node
	for _, n := range p {
		if slices.Contains(keys, n.Path[0].Key) {
			out = append(out, n)
		}
	}
	return out, nil
}
func (p setIndexPages) Paths(_ context.Context, _ uuid.UUID, ids []uuid.UUID) ([]domain.Node, error) {
	var out []domain.Node
	for _, n := range p {
		if slices.Contains(ids, n.ID) {
			out = append(out, n)
		}
	}
	return out, nil
}
func (p setIndexPages) Subtree(_ context.Context, _ uuid.UUID, id uuid.UUID) ([]domain.Step, error) {
	for _, n := range p {
		if n.ID == id {
			return n.Path, nil
		}
	}
	return nil, fmt.Errorf("fixture has no subtree %s", id)
}
func (p setIndexPages) SourcePath(_ context.Context, _ uuid.UUID, id uuid.UUID) (domain.Node, bool, error) {
	for _, n := range p {
		if n.ID == id {
			return n, true, nil
		}
	}
	return domain.Node{}, false, nil
}

type setIndexPublisher func(context.Context, app.LinksChanged) error

func (p setIndexPublisher) LinksChanged(ctx context.Context, e app.LinksChanged) error {
	return p(ctx, e)
}

// Committing chunks separately or releasing the real notebook index lock
// exposes a prefix or lets the second real writer pass before the owner ends.
func TestSetResolutionsKeepsTheIndexTransactionVisibleAtomically(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		name := "commit"
		if rollback {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			f := newSetWriteFixture(t, 8193)
			pages := setIndexPages{
				{ID: uuid.UUID{}, Path: []domain.Step{{ID: uuid.UUID{}, Key: "source0", Name: "source0"}}},
				{ID: lintID(1), Path: []domain.Step{{ID: lintID(1), Key: "source1", Name: "source1"}}},
				{ID: lintID(250), Path: []domain.Step{{ID: lintID(250), Key: "x", Name: "x"}}},
			}
			cause := errors.New("after all real Index resolution statements")
			published := 0
			index := app.Index{Store: f.s, Pages: pages, Publisher: setIndexPublisher(func(context.Context, app.LinksChanged) error {
				published++
				if rollback {
					return cause
				}
				return nil
			})}
			gate, release := make(chan setWriteCall, 1), make(chan struct{})
			let := sync.OnceFunc(func() { close(release) })
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			f.trace.after = func(ctx context.Context, i int, c setWriteCall) {
				if i == 0 && c.Err == nil {
					gate <- c
					select {
					case <-release:
					case <-ctx.Done():
					}
				}
			}
			first, firstPID, firstDone := make(chan error, 1), make(chan uint32, 1), make(chan struct{})
			go func() {
				defer close(firstDone)
				first <- f.tx.WithinTx(ctx, func(ctx context.Context) error {
					tx, _ := postgres.TxFrom(ctx)
					firstPID <- tx.Conn().PgConn().PID()
					return index.PagesChanged(ctx, app.PagesChanged{NotebookID: f.eng, Changes: []domain.Change{{NodeID: lintID(250), After: &domain.Place{Name: "x"}, Revision: 1, Facts: domain.Facts{FrontmatterValid: true}}}})
				})
			}()
			defer func() { let(); cancel(); <-firstDone }()
			var prefix setWriteCall
			select {
			case prefix = <-gate:
			case err := <-first:
				t.Fatalf("Index did not reach first completed Set: %v", err)
			case <-ctx.Done():
				t.Fatal("Index fixture did not reach its real SQL barrier")
			}
			owner := <-firstPID
			if prefix.PID != owner || prefix.Err != nil || prefix.Rows < 1 {
				t.Fatalf("first actual write=%+v owner=%d", prefix, owner)
			}
			if prefix.Lengths != [5]int{256, 256, 256, 256, 256} || prefix.Rows != 256 {
				// Keep exercising the original transaction on the RED baseline;
				// an oversized real statement is the uncalibrated private strategy.
				t.Errorf("SET_WRITE_BATCHES: first completed statement=%+v, want exactly 256 rows", prefix)
			}
			reader, err := f.pool.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			readerPID := reader.Conn().PgConn().PID()
			visible, readErr := f.s.Links(postgres.OnConn(ctx, reader), f.eng, domain.Reach{Sources: []uuid.UUID{{}, lintID(1)}})
			reader.Release()
			if readerPID == owner || readErr != nil || !slices.Equal(visible, f.before) {
				t.Fatalf("outside reader saw a prefix: owner=%d reader=%d rows=%d err=%v", owner, readerPID, len(visible), readErr)
			}
			second, secondPID, secondDone := make(chan error, 1), make(chan uint32, 1), make(chan struct{})
			go func() {
				defer close(secondDone)
				second <- f.tx.WithinTx(ctx, func(ctx context.Context) error {
					tx, _ := postgres.TxFrom(ctx)
					secondPID <- tx.Conn().PgConn().PID()
					if err := f.s.Lock(ctx, f.eng); err != nil {
						return err
					}
					_, err := postgres.DB(ctx, f.pool).Exec(ctx, "UPDATE indexed_pages SET revision=revision+1 WHERE node_id=$1", lintID(1))
					return err
				})
			}()
			defer func() { let(); cancel(); <-secondDone }()
			var waiter uint32
			select {
			case waiter = <-secondPID:
			case <-ctx.Done():
				t.Fatal("second writer did not begin")
			}
			pgtest.WaitForLockWaits(t, f.pool, 1, 10*time.Second)
			var blocked bool
			err = f.pool.QueryRow(ctx, `SELECT $1::integer=ANY(pg_blocking_pids($2::integer)) AND EXISTS (
 SELECT 1 FROM pg_locks a JOIN pg_locks b ON a.database=b.database AND a.classid=b.classid AND a.objid=b.objid AND a.objsubid=b.objsubid
 WHERE a.pid=$1 AND b.pid=$2 AND a.locktype='advisory' AND b.locktype='advisory' AND a.granted AND NOT b.granted)`, owner, waiter).Scan(&blocked)
			if err != nil || !blocked || owner == waiter {
				t.Fatalf("real same index lock not held: owner=%d waiter=%d blocked=%t err=%v", owner, waiter, blocked, err)
			}
			select {
			case err := <-second:
				t.Fatalf("second writer escaped original index lock: %v", err)
			default:
			}
			let()
			var firstErr, secondErr error
			select {
			case firstErr = <-first:
			case <-ctx.Done():
				t.Fatal("first writer did not finish after release")
			}
			select {
			case secondErr = <-second:
			case <-ctx.Done():
				t.Fatal("second writer did not finish after owner ended")
			}
			if rollback && !errors.Is(firstErr, cause) || !rollback && firstErr != nil || secondErr != nil || published != 1 {
				t.Fatalf("Index/second writer termination: rollback=%t first=%v second=%v publish=%d", rollback, firstErr, secondErr, published)
			}
			want := slices.Clone(f.before)
			if !rollback {
				for i := range want {
					want[i].Resolution = domain.Resolution{ID: lintID(250)}
				}
			}
			f.assertRows(t, want)
			var revision int
			if err := f.pool.QueryRow(t.Context(), "SELECT revision FROM indexed_pages WHERE node_id=$1", lintID(1)).Scan(&revision); err != nil || revision != 2 {
				t.Fatalf("second writer never committed: %d %v", revision, err)
			}
			for i := range f.desired {
				f.desired[i].Resolution = domain.Resolution{ID: lintID(250)}
			}
			lengths := setWrite256Lengths(32, 1)
			for i, c := range f.assertCalls(t, f.desired, lengths, owner) {
				if c.Err != nil || c.Rows != int64(lengths[i]) {
					t.Fatalf("Index actual completed statements: %+v", c)
				}
			}
			t.Logf("SETWRITE atomic Index: owner=%d waiter=%d reader=%d actual advisory blocker=true rollback=%t complete_rows=8193", owner, waiter, readerPID, rollback)
		})
	}
}
