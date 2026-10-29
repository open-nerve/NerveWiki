package postgresadapter_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

func sourceSum(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

func (f fixture) sourceHash(t *testing.T, id uuid.UUID) []byte {
	t.Helper()
	var hash []byte
	if err := f.pool.QueryRow(context.Background(), "SELECT source_hash FROM nodes WHERE id = $1", id).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	return hash
}

// Heads preserves the previous derived hash of both kinds, reads deleted
// content without its text, and never manufactures a missing or foreign node.
func TestSourceHeadsReadBoundedMetadata(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	live := f.page(t, f.eng, nil, "Live", 0)
	trash := f.page(t, f.eng, nil, "Trash", 1)
	asset := f.asset(t, f.eng, nil, "file.txt", 2)
	other := f.page(t, f.ops, nil, "Other", 0)
	for _, n := range []domain.Node{live, trash} {
		body := n.Name + " 正文"
		f.exec(t, "UPDATE page_contents SET content = $2, content_hash = $3, byte_size = $4 WHERE node_id = $1",
			n.ID, body, sourceSum(body), len(body))
	}
	f.exec(t, "UPDATE nodes SET source_hash = $2 WHERE id = $1", live.ID, sourceSum("old page"))
	f.exec(t, "UPDATE nodes SET source_hash = $2 WHERE id = $1", asset.ID, sourceSum("old asset"))
	f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", trash.ID, now())
	f.exec(t, "UPDATE page_contents SET deleted_at = $2 WHERE node_id = $1", trash.ID, now())
	got, err := f.s.SourceHeads(ctx, f.eng, []uuid.UUID{asset.ID, other.ID, trash.ID, uuid.NewV7(), live.ID})
	if err != nil || len(got) != 3 {
		t.Fatalf("SourceHeads = %+v, %v; want live/trash pages and asset, no missing or foreign node", got, err)
	}
	want := map[uuid.UUID]app.SourceValue{
		live.ID:  {ID: live.ID, Kind: domain.KindPage, Hash: sourceSum("Live 正文"), BeforeSourceHash: sourceSum("old page"), Bytes: int64(len("Live 正文"))},
		trash.ID: {ID: trash.ID, Kind: domain.KindPage, Hash: sourceSum("Trash 正文"), Bytes: int64(len("Trash 正文"))},
		asset.ID: {ID: asset.ID, Kind: domain.KindAsset, BeforeSourceHash: sourceSum("old asset")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SourceHeads = %+v, want %+v", got, want)
	}
	failure := errors.New("roll back source metadata")
	err = postgres.NewTxManager(f.pool, time.Second).WithinTx(ctx, func(ctx context.Context) error {
		if _, err := postgres.DB(ctx, f.pool).Exec(ctx, "UPDATE nodes SET source_hash = $2 WHERE id = $1", asset.ID, sourceSum("uncommitted")); err != nil {
			return err
		}
		got, err := f.s.SourceHeads(ctx, f.eng, []uuid.UUID{asset.ID})
		if err != nil || !bytes.Equal(got[asset.ID].BeforeSourceHash, sourceSum("uncommitted")) {
			t.Errorf("SourceHeads in transaction = %+v, %v; want its uncommitted asset hash", got, err)
		}
		return failure
	})
	if !errors.Is(err, failure) || !bytes.Equal(f.sourceHash(t, asset.ID), sourceSum("old asset")) {
		t.Errorf("metadata rollback = %v, hash %x; want old asset hash", err, f.sourceHash(t, asset.ID))
	}
}

// Explicit UUID endpoints catch an exclusive first lower bound and cursor
// repetition. Probe order differs from ID order; page/asset and notebooks
// have equal hashes so either missing namespace filter changes the answer.
func TestSourceProbesFollowBoundedCursors(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	sum := sha256.Sum256([]byte("same key"))
	minID, midID, maxID := uuid.Nil(), uuid.MustParse("80000000-0000-0000-0000-000000000000"), uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	assetID := uuid.MustParse("40000000-0000-0000-0000-000000000000")
	for i, row := range []struct {
		id, notebook uuid.UUID
		kind         string
		deleted      bool
	}{
		{minID, f.eng, "page", false}, {midID, f.eng, "page", false}, {maxID, f.eng, "page", false},
		{assetID, f.eng, "asset", false}, {uuid.NewV7(), f.ops, "page", false}, {uuid.NewV7(), f.eng, "page", true},
	} {
		var gone *time.Time
		if row.deleted {
			at := now()
			gone = &at
		}
		f.exec(t, `INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, source_hash,
			created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			VALUES ($1, $2, $3, $4, $4, $5, $6, $7, $7, $8, $8, $9)`,
			row.id, row.notebook, row.kind, fmt.Sprintf("probe-%d", i), i, sum[:], f.alice, now(), gone)
	}
	pageKey, assetKey := app.SourceKey{Kind: domain.KindPage, Hash: sum}, app.SourceKey{Kind: domain.KindAsset, Hash: sum}
	for _, tt := range []struct {
		name   string
		probes []app.SourceProbe
		want   []app.SourceMatch
	}{
		{"first includes minimum", []app.SourceProbe{{Key: assetKey, Take: 1}, {Key: pageKey, Take: 2}},
			[]app.SourceMatch{{Probe: 0, ID: assetID}, {Probe: 1, ID: minID}, {Probe: 1, ID: midID}}},
		{"cursor excludes itself", []app.SourceProbe{{Key: pageKey, After: &minID, Take: 2}},
			[]app.SourceMatch{{Probe: 0, ID: midID}, {Probe: 0, ID: maxID}}},
		{"last is included", []app.SourceProbe{{Key: pageKey, After: &midID, Take: 1}}, []app.SourceMatch{{Probe: 0, ID: maxID}}},
		{"past maximum", []app.SourceProbe{{Key: pageKey, After: &maxID, Take: 1}}, nil},
		{"missing hash", []app.SourceProbe{{Key: app.SourceKey{Kind: domain.KindPage}, Take: 1}}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.s.ProbeSources(ctx, f.eng, tt.probes)
			if err != nil || !slices.Equal(got, tt.want) {
				t.Errorf("ProbeSources = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
	failure := errors.New("rollback probe")
	err := postgres.NewTxManager(f.pool, time.Second).WithinTx(ctx, func(ctx context.Context) error {
		if _, err := postgres.DB(ctx, f.pool).Exec(ctx, "UPDATE nodes SET source_hash = $2 WHERE id = $1", midID, sourceSum("other")); err != nil {
			return err
		}
		got, err := f.s.ProbeSources(ctx, f.eng, []app.SourceProbe{{Key: pageKey, After: &minID, Take: 1}})
		if err != nil || !slices.Equal(got, []app.SourceMatch{{ID: maxID}}) {
			t.Errorf("probe sees transaction = %+v, %v; want only maximum", got, err)
		}
		return failure
	})
	if !errors.Is(err, failure) || !bytes.Equal(f.sourceHash(t, midID), sum[:]) {
		t.Errorf("probe transaction rollback = %v, hash %x; want original", err, f.sourceHash(t, midID))
	}
}

func TestSourceHashesSetAndForgetInTransaction(t *testing.T) {
	for _, operation := range []string{"set", "forget"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			f := newFixture(t)
			one := f.page(t, f.eng, nil, "One", 0)
			two := f.asset(t, f.eng, nil, "two.txt", 1)
			trash := f.page(t, f.eng, nil, "Trash", 2)
			foreign := f.page(t, f.ops, nil, "Foreign", 0)
			oldHash := sourceSum("old")
			for _, n := range []domain.Node{one, two, trash, foreign} {
				f.exec(t, "UPDATE nodes SET source_hash = $2 WHERE id = $1", n.ID, oldHash)
			}
			// A deliberate invalid trash hash checks the port's live predicate;
			// it is not presented as a valid full-tree fixture.
			f.exec(t, "UPDATE nodes SET deleted_at = $2 WHERE id = $1", trash.ID, now())
			write := func(ctx context.Context) error {
				if operation == "forget" {
					return f.s.ForgetSources(ctx, f.eng)
				}
				_, err := f.s.SetSourceHashes(ctx, f.eng, []app.SourceValue{
					{ID: one.ID, Hash: sourceSum("new")}, {ID: two.ID}, {ID: trash.ID}, {ID: foreign.ID},
				})
				return err
			}
			check := func(ctx context.Context, changed bool) {
				t.Helper()
				for _, n := range []domain.Node{one, two, trash, foreign} {
					want := oldHash
					if changed && (n.ID == one.ID || n.ID == two.ID) {
						want = nil
						if operation == "set" && n.ID == one.ID {
							want = sourceSum("new")
						}
					}
					var got []byte
					if err := postgres.DB(ctx, f.pool).QueryRow(ctx, "SELECT source_hash FROM nodes WHERE id = $1", n.ID).Scan(&got); err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(got, want) || (got == nil) != (want == nil) {
						t.Errorf("%s node %s changed=%t: hash %x, want %x", operation, n.Name, changed, got, want)
					}
				}
			}
			failure := errors.New("rollback hash write")
			err := postgres.NewTxManager(f.pool, time.Second).WithinTx(ctx, func(ctx context.Context) error {
				if err := write(ctx); err != nil {
					return err
				}
				check(ctx, true)
				return failure
			})
			if !errors.Is(err, failure) {
				t.Fatalf("rollback = %v, want marker error", err)
			}
			check(ctx, false)
			if err := write(ctx); err != nil {
				t.Fatal(err)
			}
			check(ctx, true)
			// A second identical Set must not create a replacement tuple.
			var before, after string
			if err := f.pool.QueryRow(ctx, "SELECT ctid::text FROM nodes WHERE id = $1", one.ID).Scan(&before); err != nil {
				t.Fatal(err)
			}
			if err := write(ctx); err != nil {
				t.Fatal(err)
			}
			if err := f.pool.QueryRow(ctx, "SELECT ctid::text FROM nodes WHERE id = $1", one.ID).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Errorf("unchanged %s replaced tuple %s with %s", operation, before, after)
			}
		})
	}
}

// Nil pools make any attempted query fail loudly: empties and invalid bounds
// must be decided before SQL. The caller, not SQL, splits larger workloads.
func TestSourcePortsKeepTheirBounds(t *testing.T) {
	ctx := context.Background()
	s := postgresadapter.New(nil)
	id := uuid.NewV7()
	if got, err := s.SourceHeads(ctx, id, nil); err != nil || len(got) != 0 {
		t.Errorf("empty heads = %v, %v", got, err)
	}
	if got, err := s.SourceBaselines(ctx, id, nil); err != nil || len(got) != 0 {
		t.Errorf("empty baselines=%v/%v", got, err)
	}
	if _, err := s.SourceBaselines(ctx, id, make([]uuid.UUID, domain.MaxUnitNodes+1)); err == nil {
		t.Error("unbounded baselines accepted")
	}
	if got, err := s.ProbeSources(ctx, id, nil); err != nil || len(got) != 0 {
		t.Errorf("empty probes = %v, %v", got, err)
	}
	if _, err := s.SetSourceHashes(ctx, id, nil); err != nil {
		t.Errorf("empty set = %v", err)
	}
	if _, err := s.SourceHeads(ctx, id, make([]uuid.UUID, domain.MaxUnitNodes+1)); err == nil {
		t.Error("10001 heads accepted, want a bounded request error")
	}
	if _, err := s.SetSourceHashes(ctx, id, make([]app.SourceValue, domain.MaxUnitNodes+1)); err == nil {
		t.Error("10001 hash writes accepted, want a bounded request error")
	}
	for _, length := range []int{0, 31, 33} {
		if _, err := s.SetSourceHashes(ctx, id, []app.SourceValue{{ID: id, Hash: make([]byte, length)}}); err == nil {
			t.Errorf("nonNULL hash of %d bytes accepted", length)
		}
	}
	key := app.SourceKey{Kind: domain.KindPage}
	for _, probes := range [][]app.SourceProbe{
		{{Key: key, Take: 0}}, {{Key: key, Take: -1}}, {{Key: key, Take: domain.MaxUnitNodes + 1}},
		{{Key: key, Take: domain.MaxUnitNodes}, {Key: key, Take: 1}}, make([]app.SourceProbe, domain.MaxUnitNodes+1),
	} {
		if _, err := s.ProbeSources(ctx, id, probes); err == nil {
			t.Errorf("invalid probe bounds (%d probes) accepted", len(probes))
		}
	}
}

type sourceQueryTrace struct {
	heads []string
}

func (q *sourceQueryTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, "-- name: SourceHeads ") {
		q.heads = append(q.heads, data.SQL)
	}
	return ctx
}

func (*sourceQueryTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestSourceQueriesKeepTheirPlans(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	n := f.page(t, f.eng, nil, "Source", 0)
	hash := sha256.Sum256([]byte("source"))
	f.exec(t, "UPDATE nodes SET source_hash = $2 WHERE id = $1", n.ID, hash[:])
	trace := &sourceQueryTrace{}
	pc := f.pool.Config()
	pc.MaxConns, pc.ConnConfig.Tracer = 1, trace
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := postgresadapter.New(pool)
	for range 8 {
		if got, err := s.SourceHeads(ctx, f.eng, []uuid.UUID{n.ID}); err != nil || len(got) != 1 {
			t.Fatalf("heads = %v, %v, want source", got, err)
		}
		if got, err := s.ProbeSources(ctx, f.eng, []app.SourceProbe{{Key: app.SourceKey{Kind: domain.KindPage, Hash: hash}, Take: 1}}); err != nil || len(got) != 1 {
			t.Fatalf("probe = %v, %v, want source", got, err)
		}
		if _, err := s.SetSourceHashes(ctx, f.eng, []app.SourceValue{{ID: n.ID, Hash: hash[:]}}); err != nil {
			t.Fatal(err)
		}
		if got, err := s.SourceBaselines(ctx, f.eng, []uuid.UUID{n.ID}); err != nil || len(got) != 1 {
			t.Fatalf("baseline=%v/%v", got, err)
		}
		if _, err := s.FindNodeIn(ctx, f.eng, n.ID); err != nil {
			t.Fatal(err)
		}
	}
	cached := pgtest.CachedStatements(t, pool)
	for _, name := range []string{"SourceHeads", "ProbeSources", "SetSourceHashes", "SourceBaselines"} {
		if slices.Contains(cached, name) {
			t.Errorf("variable source query %s cached: %v", name, cached)
		}
	}
	if !slices.Contains(cached, "FindNodeIn") {
		t.Errorf("cache positive control FindNodeIn missing: %v", cached)
	}
	if len(trace.heads) != 8 {
		t.Fatalf("traced %d heads queries, want all 8 actual executions", len(trace.heads))
	}
	projection := regexp.MustCompile(`(?is)\bselect\b(.*?)\bfrom\b`)
	body := regexp.MustCompile(`(?i)\bcontent\b|\*`)
	for _, query := range trace.heads {
		selected := projection.FindStringSubmatch(query)
		if len(selected) != 2 || body.MatchString(selected[1]) {
			t.Errorf("executed SourceHeads projected page bodies or a wildcard: %q", query)
		}
	}
}

func TestSourceLockRequiresTransactionAndSerializes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	if err := f.s.LockSources(ctx, f.eng); err == nil {
		t.Fatal("source lock accepted a context outside a transaction")
	}
	locked, release, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	defer close(release)
	tx := postgres.NewTxManager(f.pool, time.Second)
	go func() {
		finished <- tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := f.s.LockSources(ctx, f.eng); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	select {
	case <-locked:
	case err := <-finished:
		t.Fatalf("holder failed before locking: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("holder did not take source lock")
	}
	// A different notebook does not queue behind the first one's lock.
	otherCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := tx.WithinTx(otherCtx, func(ctx context.Context) error { return f.s.LockSources(ctx, f.ops) }); err != nil {
		t.Fatalf("unrelated notebook source lock: %v", err)
	}
	waitCtx, stop := context.WithCancel(ctx)
	defer stop()
	waiter := make(chan error, 1)
	go func() {
		waiter <- tx.WithinTx(waitCtx, func(ctx context.Context) error { return f.s.LockSources(ctx, f.eng) })
	}()
	h := fnv.New32a()
	_, _ = h.Write(f.eng[:])
	pgtest.WaitForAdvisoryLockWaits(t, f.pool, 0x73726365, int32(h.Sum32()), 1, 3*time.Second)
	// End the holder without waiting for the deferred close, then observe
	// that the waiting transaction acquires the same lock and completes.
	release <- struct{}{}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-waiter:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("source lock did not release at commit")
	}
}

func TestSourceWritesReturnOnlyActualOldKeys(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	page := f.page(t, f.eng, nil, "Page", 0)
	empty := f.page(t, f.eng, nil, "Empty", 1)
	asset := f.asset(t, f.eng, nil, "file.bin", 2)
	foreign := f.page(t, f.ops, nil, "Foreign", 0)
	trash := f.page(t, f.eng, nil, "Trash", 3)
	old, newHash := sourceSum("old"), sourceSum("new")
	for _, id := range []uuid.UUID{page.ID, asset.ID, foreign.ID, trash.ID} {
		f.exec(t, "UPDATE nodes SET source_hash=$2 WHERE id=$1", id, old)
	}
	f.exec(t, "UPDATE nodes SET deleted_at=$2 WHERE id=$1", trash.ID, now())
	ids := []uuid.UUID{page.ID, empty.ID, asset.ID, foreign.ID, trash.ID, uuid.NewV7()}
	baselines, err := f.s.SourceBaselines(ctx, f.eng, ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(baselines) != 3 || baselines[empty.ID].BeforeSourceHash != nil || baselines[asset.ID].Kind != domain.KindAsset || !bytes.Equal(baselines[page.ID].BeforeSourceHash, old) {
		t.Fatalf("derived-only baselines=%+v", baselines)
	}
	values := []app.SourceValue{{ID: page.ID, Hash: newHash}, {ID: asset.ID}, {ID: empty.ID, Hash: newHash}, {ID: foreign.ID}, {ID: trash.ID}}
	marker := errors.New("rollback source preimages")
	err = postgres.NewTxManager(f.pool, time.Second).WithinTx(ctx, func(ctx context.Context) error {
		changed, err := f.s.SetSourceHashes(ctx, f.eng, values)
		if err != nil {
			return err
		}
		if len(changed) != 3 || !bytes.Equal(changed[page.ID].BeforeSourceHash, old) || !bytes.Equal(changed[asset.ID].BeforeSourceHash, old) || changed[empty.ID].BeforeSourceHash != nil || changed[asset.ID].Kind != domain.KindAsset {
			t.Errorf("actual old keys=%+v", changed)
		}
		again, err := f.s.SetSourceHashes(ctx, f.eng, values)
		if err != nil {
			return err
		}
		if len(again) != 0 {
			t.Errorf("no-op returned preimages=%+v", again)
		}
		return marker
	})
	if !errors.Is(err, marker) {
		t.Fatal(err)
	}
	if !bytes.Equal(f.sourceHash(t, page.ID), old) || f.sourceHash(t, empty.ID) != nil || !bytes.Equal(f.sourceHash(t, asset.ID), old) {
		t.Error("rollback did not restore old keys")
	}
}
