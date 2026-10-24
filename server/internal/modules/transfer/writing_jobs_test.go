package transfer_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The writing jobs on a real database (M10/P1 design 3.10): a notebook's
// import, trash restore or revert, queued or running, anyone's, is one; an
// export, a job ended or deleted, and another notebook's are none. The
// read joins the caller's transaction: a job written there, not committed,
// is one, and gone with its rollback.
func TestTheWritingJobsAreTheTreesWriters(t *testing.T) {
	r := newRoot(t)
	ctx := context.Background()
	jobs := transfer.NewWritingJobs(r.pool)
	beat := time.Now()
	end := func(id uuid.UUID, set string) {
		if _, err := r.pool.Exec(ctx, "UPDATE transfer_jobs SET "+set+" WHERE id = $1", id); err != nil {
			t.Fatal(err)
		}
	}
	busy := r.notebook(t)
	r.jobOf(t, r.alice, domain.KindImport, "queued", beat, busy)
	for _, c := range []struct {
		name string
		add  func(notebook uuid.UUID)
		want bool
	}{
		{"none", func(uuid.UUID) {}, false},
		{"an import queued", func(nb uuid.UUID) { r.jobOf(t, r.alice, domain.KindImport, "queued", beat, nb) }, true},
		{"an import running", func(nb uuid.UUID) { r.jobOf(t, r.alice, domain.KindImport, "running", beat, nb) }, true},
		{"a trash restore running", func(nb uuid.UUID) { r.jobOf(t, r.alice, domain.KindTrashRestore, "running", beat, nb) }, true},
		{"a revert queued", func(nb uuid.UUID) { r.jobOf(t, r.alice, domain.KindRevert, "queued", beat, nb) }, true},
		{"an export queued", func(nb uuid.UUID) { r.jobOf(t, r.alice, domain.KindExport, "queued", beat, nb) }, false},
		{"an import ended", func(nb uuid.UUID) {
			end(r.jobOf(t, r.alice, domain.KindImport, "running", beat, nb), "state = 'failed', finished_at = now(), report = '{}'")
		}, false},
		{"an import deleted", func(nb uuid.UUID) {
			end(r.jobOf(t, r.alice, domain.KindImport, "queued", beat, nb), "deleted_at = now()")
		}, false},
	} {
		nb := r.notebook(t)
		c.add(nb)
		if got, err := jobs.Writing(ctx, nb); err != nil || got != c.want {
			t.Errorf("%s: Writing() = %v, %v; want %v", c.name, got, err, c.want)
		}
	}

	nb := r.notebook(t)
	rollback := errors.New("rolled back")
	var inside bool
	err := postgres.NewTxManager(r.pool, 5*time.Second).WithinTx(ctx, func(ctx context.Context) error {
		if _, err := postgres.DB(ctx, r.pool).Exec(ctx, `INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client,
			created_at, changeset_id) VALUES ($1, $2, 'import', 'queued', 'Eng', $3, 'api', now(), $4)`, uuid.NewV7(), nb, r.alice,
			uuid.NewV7()); err != nil {
			return err
		}
		var err error
		if inside, err = jobs.Writing(ctx, nb); err != nil {
			return err
		}
		return rollback
	})
	if outside, werr := jobs.Writing(ctx, nb); !errors.Is(err, rollback) || !inside || werr != nil || outside {
		t.Errorf("in the transaction %v (%v), after its rollback %v (%v); want true, then false", inside, err, outside, werr)
	}
}
