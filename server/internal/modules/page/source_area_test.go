package page_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The pool-only module port must find the caller's transaction: a later
// mode subscriber's failure has to restore the source hash too.
func TestSourceAreaForgetUsesTheCallerTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	source := uuid.NewV7()
	hash := sha256.Sum256([]byte("source"))
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order,
		created_by_id, updated_by_id, created_at, updated_at, source_hash)
		VALUES ($1, $2, $3, 'page', 'Source', 'source', 0, $4, $4, $5, $5, $6)`, source, f.eng, f.notes, f.alice, testNow(), hash[:])
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		VALUES ($1, 'source', 1, $2, 6, $3, $4)`, source, hash[:], f.alice, testNow())
	f.exec(t, "UPDATE notebooks SET mode = 'llm_wiki', source_root_id = $2 WHERE id = $1", f.eng, f.notes)
	area := page.NewSourceArea(f.pool)
	read := func(ctx context.Context) []byte {
		t.Helper()
		var got []byte
		if err := postgres.DB(ctx, f.pool).QueryRow(ctx, "SELECT source_hash FROM nodes WHERE id = $1", source).Scan(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	failure := errors.New("a later subscriber failed")
	err := postgres.NewTxManager(f.pool, time.Second).WithinTx(ctx, func(ctx context.Context) error {
		if err := area.Forget(ctx, f.eng); err != nil {
			return err
		}
		if got := read(ctx); got != nil {
			t.Errorf("Forget in caller transaction left hash %x, want NULL", got)
		}
		return failure
	})
	if !errors.Is(err, failure) || !bytes.Equal(read(ctx), hash[:]) {
		t.Errorf("Forget rollback = %v, hash %x; want original source hash", err, read(ctx))
	}
	if err := area.Forget(ctx, f.eng); err != nil {
		t.Fatal(err)
	}
	if got := read(ctx); got != nil {
		t.Errorf("committed Forget left hash %x", got)
	}
}

// The public key used by interleaving tests must identify the lock the
// source store actually holds, including signed second keys.
func TestSourceLockKeyNamesTheActualLock(t *testing.T) {
	f := newFixture(t)
	s := postgresadapter.New(f.pool)
	err := postgres.NewTxManager(f.pool, time.Second).WithinTx(context.Background(), func(ctx context.Context) error {
		if err := s.LockSources(ctx, f.eng); err != nil {
			return err
		}
		var count int
		if err := postgres.DB(ctx, f.pool).QueryRow(ctx, `SELECT count(*) FROM pg_locks
			WHERE pid = pg_backend_pid() AND locktype = 'advisory' AND granted AND objsubid = 2
			AND classid::bigint = $1 AND objid::bigint = $2`, int64(0x73726365), int64(uint32(page.SourceLockKey(f.eng)))).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			t.Errorf("public source key found %d held locks, want 1", count)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
