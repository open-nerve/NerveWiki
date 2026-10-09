package app

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The jobs' upkeep (M7/P5 design 3.12): the exports expire, the jobs that
// stopped beating are failed, the archives no job keeps are swept, the
// deleted notebooks' jobs follow them, the purge deletes their rows.

// expireBatch is how many exports one statement expires.
const expireBatch = 100

// Expire expires the exports that succeeded longer than the TTL ago:
// their archives are deleted.
type Expire struct {
	rows     MaintainedRows
	archives Archives
	clock    Clock
	logger   *slog.Logger
	ttl      time.Duration
}

// NewExpire returns the use case, ttl transfer.export_ttl.
func NewExpire(rows MaintainedRows, archives Archives, clock Clock, logger *slog.Logger, ttl time.Duration) *Expire {
	return &Expire{rows: rows, archives: archives, clock: clock, logger: logger, ttl: ttl}
}

// Run expires them, expireBatch a statement, and deletes their archives
// once expired: an archive it cannot delete is logged, left to the sweep.
// It tells how many it expired.
func (e *Expire) Run(ctx context.Context) (int, error) {
	before := e.clock.Now().Add(-e.ttl)
	expired := 0
	for {
		ids, err := e.rows.ExpireExports(ctx, before, expireBatch)
		if err != nil {
			return expired, err
		}
		expired += len(ids)
		for _, id := range ids {
			if err := e.archives.Delete(ctx, domain.KindExport, id); err != nil {
				e.logger.WarnContext(ctx, "expired export's archive not deleted", slog.String("job_id", id.String()), slog.Any("error", err))
			}
		}
		if len(ids) < expireBatch {
			break
		}
	}
	if expired > 0 {
		e.logger.InfoContext(ctx, "exports expired", slog.Int("jobs", expired))
	}
	return expired, nil
}

// Rescue fails the jobs that no longer run, or will not, though their rows
// say so: the server's own running ones, as it starts, which the last
// process left; then those whose heartbeat is older than the timeout, and
// the queued exports River no longer holds. A job does not run again. Its
// report tells the failure alone; its progress, how far it went.
type Rescue struct {
	rows    MaintainedRows
	held    Held
	clock   Clock
	logger  *slog.Logger
	timeout time.Duration
}

// NewRescue returns the use case, timeout transfer.heartbeat_timeout.
func NewRescue(rows MaintainedRows, held Held, clock Clock, logger *slog.Logger, timeout time.Duration) *Rescue {
	return &Rescue{rows: rows, held: held, clock: clock, logger: logger, timeout: timeout}
}

// AtStart fails every running job: as the server starts, before it runs
// any, they were all interrupted (one server). One a request holds is
// left to Run, its heartbeat old by then.
func (r *Rescue) AtStart(ctx context.Context) error {
	jobs, err := r.rows.InterruptJobs(ctx, nil, r.clock.Now(), domain.Report{Failure: domain.FailureInterrupted})
	if err != nil {
		return fmt.Errorf("rescue the interrupted jobs: %w", err)
	}
	r.log(ctx, "job interrupted", jobs)
	return nil
}

// Run fails the running jobs whose heartbeat is older than the timeout,
// then the queued exports River no longer holds: it dropped them, its one
// attempt spent before they started, as their start's write failed. One
// whose process stopped as River took it River holds, running, until its
// rescue: the job timeout and an hour (M7/P5 design 3.12).
func (r *Rescue) Run(ctx context.Context) error {
	before := r.clock.Now().Add(-r.timeout)
	jobs, err := r.rows.InterruptJobs(ctx, &before, r.clock.Now(), domain.Report{Failure: domain.FailureInterrupted})
	if err != nil {
		return fmt.Errorf("rescue the interrupted jobs: %w", err)
	}
	r.log(ctx, "job interrupted", jobs)
	return r.lost(ctx)
}

// lost fails the queued exports River no longer holds. It reads the rows
// before River: a job queued then was enqueued with its row, so River
// holds it still, or started it, the row no longer queued, or dropped it.
func (r *Rescue) lost(ctx context.Context) error {
	queued, err := r.rows.QueuedExports(ctx)
	if err != nil || len(queued) == 0 {
		return err
	}
	ids, err := r.held.Held(ctx)
	if err != nil {
		return fmt.Errorf("the jobs River holds: %w", err)
	}
	held := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		held[id] = true
	}
	var lost []uuid.UUID
	for _, id := range queued {
		if !held[id] {
			lost = append(lost, id)
		}
	}
	if len(lost) == 0 {
		return nil
	}
	jobs, err := r.rows.FailQueued(ctx, lost, r.clock.Now(), domain.Report{Failure: domain.FailureInterrupted})
	if err != nil {
		return fmt.Errorf("rescue the jobs River dropped: %w", err)
	}
	r.log(ctx, "queued job dropped by River", jobs)
	return nil
}

func (r *Rescue) log(ctx context.Context, msg string, jobs []Interrupted) {
	for _, j := range jobs {
		r.logger.WarnContext(ctx, msg, slog.String("job_id", j.ID.String()), slog.String("notebook_id", j.NotebookID.String()),
			slog.String("user_id", j.CreatedBy.String()), slog.String("client", string(j.Client)), slog.String("failure", string(domain.FailureInterrupted)))
	}
}

// SweepAge is how old an archive no job keeps is when the sweep deletes
// it: a day, longer than an export's archive waits between its commit and
// its job's success.
const SweepAge = 24 * time.Hour

// sweepBatch is how many archives one read of the rows asks about.
const sweepBatch = 500

// Sweep deletes the exports' archives no job keeps: those whose job ended
// otherwise than in success after the file committed, expired ones whose
// delete failed, a deleted notebook's.
type Sweep struct {
	rows     MaintainedRows
	archives Archives
	clock    Clock
	logger   *slog.Logger
}

// NewSweep returns the use case.
func NewSweep(rows MaintainedRows, archives Archives, clock Clock, logger *slog.Logger) *Sweep {
	return &Sweep{rows: rows, archives: archives, clock: clock, logger: logger}
}

// Run deletes them, older than SweepAge, sweepBatch a read of the rows,
// and tells how many. One it cannot delete is logged, the run going on to
// the rest and failing at its end.
func (s *Sweep) Run(ctx context.Context) (int, error) {
	deleted, failed := 0, 0
	var first error
	var batch []uuid.UUID
	flush := func() error {
		live, err := s.rows.LiveArchives(ctx, batch)
		if err != nil {
			return err
		}
		kept := make(map[uuid.UUID]bool, len(live))
		for _, id := range live {
			kept[id] = true
		}
		for _, id := range batch {
			if kept[id] {
				continue
			}
			if err := s.archives.Delete(ctx, domain.KindExport, id); err != nil {
				s.logger.WarnContext(ctx, "orphan export archive not deleted", slog.String("job_id", id.String()), slog.Any("error", err))
				failed++
				first = cmp.Or(first, err)
				continue
			}
			deleted++
		}
		batch = batch[:0]
		return nil
	}
	err := s.archives.List(ctx, domain.KindExport, s.clock.Now().Add(-SweepAge), func(id uuid.UUID) error {
		if batch = append(batch, id); len(batch) == sweepBatch {
			return flush()
		}
		return nil
	})
	if err == nil && len(batch) > 0 {
		err = flush()
	}
	if deleted > 0 {
		s.logger.InfoContext(ctx, "orphan export archives deleted", slog.Int("files", deleted))
	}
	if err == nil && failed > 0 {
		err = fmt.Errorf("%d orphan export archives not deleted, the first: %w", failed, first)
	}
	return deleted, err
}

// Follow deletes the deleted notebooks' jobs, in the deletion's
// transaction: a queued one then does nothing, a running one stops at its
// next heartbeat.
type Follow struct {
	rows MaintainedRows
}

// NewFollow returns the subscriber.
func NewFollow(rows MaintainedRows) Follow {
	return Follow{rows: rows}
}

// NotebooksDeleted deletes the notebooks' jobs at at.
func (f Follow) NotebooksDeleted(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	return f.rows.DeleteJobsOfNotebooks(ctx, ids, at)
}

// Purge deletes the jobs' rows deleted longer than the retention ago,
// their archives first (v0.1 design 13.1, item 6).
type Purge struct {
	tx       shared.TxManager
	rows     MaintainedRows
	archives Archives
	logger   *slog.Logger
}

// NewPurge returns the purge.
func NewPurge(tx shared.TxManager, rows MaintainedRows, archives Archives, logger *slog.Logger) *Purge {
	return &Purge{tx: tx, rows: rows, archives: archives, logger: logger}
}

// Batch purges up to batch rows deleted before before, in one transaction
// that holds them: their archives, one gone deleted all the same, then the
// rows. An archive not deleted fails the batch, logged, and keeps the
// rows.
func (p *Purge) Batch(ctx context.Context, before time.Time, batch int) (int, error) {
	deleted := 0
	err := p.tx.WithinTx(ctx, func(ctx context.Context) error {
		jobs, err := p.rows.ExpiredJobs(ctx, before, batch)
		if err != nil || len(jobs) == 0 {
			return err
		}
		ids := make([]uuid.UUID, 0, len(jobs))
		for id, kind := range jobs {
			if err := p.archives.Delete(ctx, kind, id); err != nil {
				p.logger.ErrorContext(ctx, "job archive not purged", slog.String("job_id", id.String()), slog.Any("error", err))
				return fmt.Errorf("delete the archive of job %s: %w", id, err)
			}
			ids = append(ids, id)
		}
		deleted, err = p.rows.DeleteJobs(ctx, ids)
		return err
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}
