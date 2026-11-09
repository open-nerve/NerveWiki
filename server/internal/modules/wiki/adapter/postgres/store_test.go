package postgresadapter_test

import (
	"context"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// fixture is a database with alice and bob, and acme's notebooks eng and
// ops.
type fixture struct {
	pool       *pgxpool.Pool
	store      *postgresadapter.Store
	alice, bob uuid.UUID
	eng, ops   uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f := fixture{pool: pool, store: postgresadapter.New(pool), alice: uuid.NewV7(), bob: uuid.NewV7(), eng: uuid.NewV7(), ops: uuid.NewV7()}
	acme := uuid.NewV7()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', now(), now()), " +
			"($2, 'bob@corp.com', 'x', 'x', now(), now())", []any{f.alice, f.bob}},
		{"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())",
			[]any{acme, f.alice}},
		{"INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES " +
			"($1, $2, 'Eng', $3, $3, now(), now()), ($4, $2, 'Ops', $3, $3, now(), now())", []any{f.eng, acme, f.alice, f.ops}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.sql, err)
		}
	}
	return f
}

// started is an upgrade of notebook as its start writes it, by by at at.
func started(notebook, by uuid.UUID, at time.Time) domain.Upgrade {
	agents := uuid.NewV7()
	return domain.Upgrade{ID: uuid.NewV7(), NotebookID: notebook, State: domain.UpgradeUpgrading, SourceRootID: uuid.NewV7(),
		SeededRoot: true, SeededAgentsID: &agents, Locale: domain.LocaleZhCN, BaselineNodes: 7, BaselinePages: 5, BaselineWanted: 10000,
		BaselineWantedCapped: true, CreatedBy: by, CreatedAt: at}
}

// An upgrade's row (M11/P1 design 3.15): what its start writes reads back
// as the notebook's upgrade under way, and only that notebook's, rolling
// back too; one that ended is under way no more; a notebook's deletion deletes its
// upgrades at its time, by its actor, another notebook's stay; the purge
// takes those deleted before its time, and no other.
func TestTheUpgradesRows(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	at := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
	eng, ops := started(f.eng, f.alice, at), started(f.ops, f.bob, at.Add(time.Minute))
	ops.SeededAgentsID, ops.SeededRoot, ops.Locale, ops.BaselineWanted, ops.BaselineWantedCapped = nil, false, domain.LocaleEn, 3, false
	for _, u := range []domain.Upgrade{eng, ops} {
		if err := f.store.Insert(ctx, u, u.CreatedBy, u.CreatedAt); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []domain.Upgrade{eng, ops} {
		got, ok, err := f.store.Current(ctx, want.NotebookID)
		if err != nil || !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("Current(%v) = %+v, %v, %v; want %+v", want.NotebookID, got, ok, err, want)
		}
	}
	if _, ok, err := f.store.Current(ctx, uuid.NewV7()); ok || err != nil {
		t.Errorf("Current(no notebook) = %v, %v; want none", ok, err)
	}
	// Rolling back, as its step sets it (S3): still under way.
	job := uuid.NewV7()
	if _, err := f.pool.Exec(ctx, `UPDATE notebook_upgrades SET state = 'rolling_back', ending = 'abandoned', rollback_job_id = $2
		WHERE notebook_id = $1`, f.eng, job); err != nil {
		t.Fatal(err)
	}
	rolling := eng
	rolling.State, rolling.Ending, rolling.RollbackJobID = domain.UpgradeRollingBack, domain.UpgradeAbandoned, &job
	if got, ok, err := f.store.Current(ctx, f.eng); err != nil || !ok || !reflect.DeepEqual(got, rolling) {
		t.Errorf("Current(eng, rolling back) = %+v, %v, %v; want %+v", got, ok, err, rolling)
	}

	if _, err := f.pool.Exec(ctx, `UPDATE notebook_upgrades SET state = 'approved', submission_id = gen_random_uuid(),
		submitted_at = now(), submitted_by_id = created_by_id, report = '{}' WHERE notebook_id = $1`, f.ops); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := f.store.Current(ctx, f.ops); ok || err != nil {
		t.Errorf("Current(ops, approved) = %v, %v; want none under way", ok, err)
	}

	deleted := at.Add(time.Hour)
	if err := f.store.SoftDeleteOf(ctx, []uuid.UUID{f.eng}, f.bob, deleted); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := f.store.Current(ctx, f.eng); ok || err != nil {
		t.Errorf("Current(eng, deleted) = %v, %v; want none", ok, err)
	}
	var engDeleted, opsDeleted int
	if err := f.pool.QueryRow(ctx, `SELECT
			count(*) FILTER (WHERE notebook_id = $1 AND deleted_at = $3 AND updated_at = $3 AND updated_by_id = $4),
			count(*) FILTER (WHERE notebook_id = $2 AND deleted_at IS NOT NULL)
		FROM notebook_upgrades`, f.eng, f.ops, deleted, f.bob).Scan(&engDeleted, &opsDeleted); err != nil {
		t.Fatal(err)
	}
	if engDeleted != 1 || opsDeleted != 0 {
		t.Errorf("eng's upgrades deleted at its deletion %d, ops' deleted %d; want 1, 0", engDeleted, opsDeleted)
	}

	if n, err := f.store.PurgeUpgrades(ctx, deleted, 10); n != 0 || err != nil {
		t.Errorf("PurgeUpgrades(before its deletion) = %d, %v; want 0", n, err)
	}
	if n, err := f.store.PurgeUpgrades(ctx, deleted.Add(time.Second), 10); n != 1 || err != nil {
		t.Errorf("PurgeUpgrades(after it) = %d, %v; want eng's 1", n, err)
	}
	var left int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM notebook_upgrades").Scan(&left); err != nil || left != 1 {
		t.Errorf("%d upgrades left, %v; want ops' 1", left, err)
	}
}
