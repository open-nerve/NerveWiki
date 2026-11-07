package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	linkapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The existing native test owns drain and final INSERT failures. These three
// positions cover metadata/current ordinary write/publisher failures, plus
// cancellation before staged resolution, without replacing the real transaction,
// parser, Store or notification.
func TestRebuildStagingOtherFailuresRollBack(t *testing.T) {
	for _, phase := range []string{"metadata", "current_ordinary", "publisher", "cpu_cancel"} {
		t.Run(phase, func(t *testing.T) {
			f := newRebuildStagingFixture(t, "standard")
			before := ingestCommittedState(t, f.image())
			listener := f.tm.originListen(t)
			limit := uint64(0)
			switch phase {
			case "metadata":
				f.installMetadataTrigger(t)
			case "current_ordinary":
				// Source fits, then Target crosses the cap. Source's seven
				// links drain before Target's original AddPage begins.
				limit = f.beforePageLimit(2)
				f.installLateTrigger(t)
			}
			r, observed := f.rebuild(t, limit)
			sentinel := errors.New("owned rebuild publisher after native notify")
			publisher := &rebuildStagingAfterPublisher{delegate: r.Index.Publisher, cause: sentinel}
			if phase == "publisher" {
				r.Index.Publisher = publisher
			}
			ctx := t.Context()
			var cancelled *rebuildStagingCancelAfterMetadata
			if phase == "cpu_cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
				cancelled = &rebuildStagingCancelAfterMetadata{RebuildWrites: r.Writes, last: f.sources[2], cancel: cancel}
				r.Writes = cancelled
			}
			observed.start()
			_, err := r.Notebook(ctx, uuid.MustParse(f.nb))
			work := observed.stop()
			observed.checkTransaction(t, false)

			readPages := 2
			if phase == "publisher" || phase == "cpu_cancel" {
				readPages = 3
			}
			observed.checkReads(t, f.ids[:readPages])
			if got := work.queries["PageContent"]; got != (rebuildSingleWriteCount{Calls: int64(readPages), Rows: int64(readPages)}) {
				t.Fatalf("REBUILD_STAGING_FAILURE_READS: phase=%s native content=%+v want%d pages once", phase, got, readPages)
			}
			if work.reads.Calls != 0 || work.updates.Calls != 0 || work.upserts != 0 {
				t.Fatalf("REBUILD_STAGING_FAILURE_CONTINUED: phase=%s readback/update/upsert after failure: %+v", phase, work)
			}

			switch phase {
			case "cpu_cancel":
				if !cancelled.didCancel || !slices.Equal(cancelled.metadata, f.ids) || !errors.Is(err, context.Canceled) || !errors.Is(ctx.Err(), context.Canceled) {
					t.Fatalf("REBUILD_STAGING_CPU_CANCEL_CAUSE: cancelled=%v metadata=%v err=%v ctx=%v", cancelled.didCancel, cancelled.metadata, err, ctx.Err())
				}
				if len(work.failures) != 0 || len(work.notifications) != 0 || work.inserts.Calls != 0 {
					t.Fatalf("REBUILD_STAGING_CPU_CANCEL_CONTINUED: native failures/notify/final insert: %+v", work)
				}
				for _, name := range []string{"LinkTargetsByKeys", "LinkTargetsByIDs", "AliasesByKeys", "InsertRebuiltLinks", "InsertLinks"} {
					if work.queries[name].Calls != 0 {
						t.Fatalf("REBUILD_STAGING_CPU_CANCEL_CONTINUED: %s ran after last metadata: %+v", name, work)
					}
				}
				for _, want := range []struct {
					name string
					rows int64
				}{{"InsertIndexedPage", 3}, {"InsertTags", 6}, {"InsertProperties", 12}, {"InsertAliases", 6}} {
					if got := work.queries[want.name]; got != (rebuildSingleWriteCount{Calls: 3, Rows: want.rows}) {
						t.Fatalf("REBUILD_STAGING_CPU_CANCEL_POSITION: %s=%+v want3/%d", want.name, got, want.rows)
					}
				}
				if len(observed.queries) < 2 || observed.queries[len(observed.queries)-2].name != "InsertAliases" || observed.queries[len(observed.queries)-2].source != f.sources[2] {
					t.Fatalf("REBUILD_STAGING_CPU_CANCEL_POSITION: actual last metadata was not immediately followed by rollback: %+v", observed.queries)
				}
				t.Logf("REBUILD_STAGING_OTHER_FAILURE phase=%s complete_metadata=3/6/12/6 native_errors=0 fresh=0 inserts=0 cause=%v", phase, err)
			case "publisher":
				// The delegate really reached successful pg_notify in the
				// index transaction; this sentinel is a controlled publisher
				// result after that call, not a PostgreSQL-generated error.
				if err != sentinel || publisher.calls != 1 || publisher.nativeErr != nil || publisher.pid == 0 || !observed.pids[publisher.pid] { //nolint:errorlint // Preserve the exact publisher error object through the real transaction manager.
					t.Fatalf("REBUILD_STAGING_PUBLISHER_CAUSE: err=%v calls=%d native=%v pid=%d actual=%v", err, publisher.calls, publisher.nativeErr, publisher.pid, observed.pids)
				}
				if len(work.failures) != 0 || len(work.notifications) != 1 || work.queries["InsertRebuiltLinks"] != (rebuildSingleWriteCount{Calls: 3, Rows: 21}) || work.inserts != work.queries["InsertRebuiltLinks"] {
					t.Fatalf("REBUILD_STAGING_PUBLISHER_POSITION: %+v", work)
				}
				for _, want := range []struct {
					name string
					rows int64
				}{{"InsertIndexedPage", 3}, {"InsertTags", 6}, {"InsertProperties", 12}, {"InsertAliases", 6}} {
					if got := work.queries[want.name]; got != (rebuildSingleWriteCount{Calls: 3, Rows: want.rows}) {
						t.Fatalf("REBUILD_STAGING_PUBLISHER_POSITION: %s=%+v want3/%d", want.name, got, want.rows)
					}
				}
				if len(observed.queries) < 2 || observed.queries[len(observed.queries)-2].name != "select pg_notify($1, $2)" {
					t.Fatalf("REBUILD_STAGING_PUBLISHER_CONTINUED: successful pg_notify was not immediately followed by rollback: %+v", observed.queries)
				}
				for _, q := range observed.queries {
					if q.err != nil {
						t.Fatalf("REBUILD_STAGING_PUBLISHER_NATIVE: native SQL failed before controlled publisher result: %v", q.err)
					}
				}
				t.Logf("REBUILD_STAGING_OTHER_FAILURE phase=%s actual_notify=1 native_errors=0 pid=%d sentinel=%v", phase, publisher.pid, err)
			default:
				constraint, message, failedSQL := "rebuild_staging_metadata_row", "owned rebuild late metadata", "InsertAliases"
				wantDetail := rebuildStagingFailureDetail{Heads: 2, Tags: 4, Properties: 8, Aliases: 2}
				wantInserts := rebuildSingleWriteCount{}
				if phase == "current_ordinary" {
					constraint, message, failedSQL = "rebuild_staging_late_row", "owned rebuild late INSERT", "InsertLinks"
					wantDetail = rebuildStagingFailureDetail{Prior: 7, Current: 6, Heads: 2, Tags: 2, Properties: 4, Aliases: 1}
					wantInserts = rebuildSingleWriteCount{Calls: 2, Rows: 7}
				}
				var cause *pgconn.PgError
				if !errors.As(err, &cause) || cause.Code != "23514" || cause.ConstraintName != constraint || cause.Message != message {
					t.Fatalf("REBUILD_STAGING_OTHER_CAUSE: phase=%s err=%v want23514/%s", phase, err, constraint)
				}
				var detail rebuildStagingFailureDetail
				if err := json.Unmarshal([]byte(cause.Detail), &detail); err != nil {
					t.Fatal(err)
				}
				if detail.PID == 0 || !observed.pids[detail.PID] {
					t.Fatalf("REBUILD_STAGING_OTHER_TX: trigger pid=%d actual=%v", detail.PID, observed.pids)
				}
				wantDetail.PID = detail.PID
				if detail != wantDetail {
					t.Fatalf("REBUILD_STAGING_OTHER_POSITION: phase=%s partial=%+v want%+v", phase, detail, wantDetail)
				}
				failed := 0
				for _, q := range observed.queries {
					if q.err != nil {
						var native *pgconn.PgError
						if q.name != failedSQL || !errors.As(q.err, &native) || native != cause { //nolint:errorlint // Preserve the exact server error object through adapter and manager.
							t.Fatalf("REBUILD_STAGING_OTHER_CAUSE: phase=%s actual SQL=%s error=%v returned=%v", phase, q.name, q.err, err)
						}
						failed++
					} else if failed > 0 && q.name != "rollback" {
						t.Fatalf("REBUILD_STAGING_FAILURE_CONTINUED: phase=%s SQL after native cause=%s", phase, q.name)
					}
				}
				if failed != 1 || len(work.failures) != 1 || len(work.notifications) != 0 || work.inserts != wantInserts {
					t.Fatalf("REBUILD_STAGING_OTHER_WORK: phase=%s failed=%d work=%+v want inserts%+v", phase, failed, work, wantInserts)
				}
				if phase == "current_ordinary" {
					var order []string
					for _, q := range observed.queries {
						if q.name == "InsertIndexedPage" {
							order = append(order, "head:"+q.source.String())
						} else if q.insert {
							order = append(order, q.name+":"+q.source.String())
						}
					}
					want := []string{"head:" + f.sources[0].String(), "InsertLinks:" + f.sources[0].String(), "head:" + f.sources[1].String(), "InsertLinks:" + f.sources[1].String()}
					if !slices.Equal(order, want) {
						t.Fatalf("REBUILD_STAGING_CURRENT_AFTER_DRAIN: actual=%v want%v", order, want)
					}
				}
				t.Logf("REBUILD_STAGING_OTHER_FAILURE phase=%s detail=%s cause=%s/%s", phase, cause.Detail, cause.Code, cause.ConstraintName)
			}
			if after := ingestCommittedState(t, f.image()); !slices.Equal(before, after) {
				t.Fatalf("REBUILD_STAGING_OTHER_ROLLBACK: phase=%s complete committed image differs:\n%s", phase, diff(before, after))
			}
			if got := ingestNotifications(t, f.image(), listener); len(got) != 0 {
				t.Fatalf("REBUILD_STAGING_OTHER_EVENT: phase=%s committed notifications=%s", phase, got)
			}
			f.checkLiteralIndex(t)
			checkPages(t, f.tm.pool)
			checkAssets(t, f.tm.pool, f.tm.storage)
		})
	}
}

type rebuildStagingFailureDetail struct {
	PID                              uint32
	Prior, Current, Resolved         int
	Heads, Tags, Properties, Aliases int
	Future                           int
}

func (f rebuildStagingFixture) installMetadataTrigger(t *testing.T) {
	t.Helper()
	ingestExec(t, f.tm, fmt.Sprintf(`CREATE FUNCTION rebuild_staging_metadata_fail() RETURNS trigger LANGUAGE plpgsql AS $body$
	BEGIN
	 IF NEW.source_id='%s'::uuid AND NEW.alias_key='twin' AND NEW.alias='Twin' THEN
	  RAISE EXCEPTION 'owned rebuild late metadata' USING ERRCODE='23514', CONSTRAINT='rebuild_staging_metadata_row',
	   DETAIL=json_build_object('PID',pg_backend_pid(),
	    'Prior',(SELECT count(*) FROM page_links WHERE source_id='%s'::uuid),
	    'Current',(SELECT count(*) FROM page_links WHERE source_id=NEW.source_id),
	    'Resolved',(SELECT count(*) FROM page_links WHERE source_id=NEW.source_id AND resolved_id IS NOT NULL),
	    'Heads',(SELECT count(*) FROM indexed_pages WHERE notebook_id=NEW.notebook_id),
	    'Tags',(SELECT count(*) FROM page_tags WHERE notebook_id=NEW.notebook_id),
	    'Properties',(SELECT count(*) FROM page_properties WHERE notebook_id=NEW.notebook_id),
	    'Aliases',(SELECT count(*) FROM page_aliases WHERE notebook_id=NEW.notebook_id),
	    'Future',(SELECT count(*) FROM indexed_pages WHERE node_id='%s'::uuid))::text;
	 END IF;
	 RETURN NEW;
	END $body$`, f.sources[1], f.sources[0], f.sources[2]))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), interleavingWait)
		defer cancel()
		_, first := f.tm.pool.Exec(ctx, "DROP TRIGGER IF EXISTS rebuild_staging_metadata_fail ON page_aliases")
		_, second := f.tm.pool.Exec(ctx, "DROP FUNCTION IF EXISTS rebuild_staging_metadata_fail()")
		if err := errors.Join(first, second); err != nil {
			t.Errorf("owned rebuild metadata trigger cleanup: %v", err)
		}
	})
	ingestExec(t, f.tm, "CREATE TRIGGER rebuild_staging_metadata_fail BEFORE INSERT ON page_aliases FOR EACH ROW EXECUTE FUNCTION rebuild_staging_metadata_fail()")
}

type rebuildStagingAfterPublisher struct {
	delegate  linkapp.Publisher
	cause     error
	calls     int
	pid       uint32
	nativeErr error
}

func (p *rebuildStagingAfterPublisher) LinksChanged(ctx context.Context, event linkapp.LinksChanged) error {
	p.calls++
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		return errors.New("owned rebuild publisher has no real transaction")
	}
	p.pid = tx.Conn().PgConn().PID()
	p.nativeErr = p.delegate.LinksChanged(ctx, event)
	if p.nativeErr != nil {
		return p.nativeErr
	}
	return p.cause
}

// This wrapper waits for the original metadata write to return successfully.
// It performs no SQL and changes neither the rows nor the error returned by it.
type rebuildStagingCancelAfterMetadata struct {
	linkapp.RebuildWrites
	last      uuid.UUID
	cancel    context.CancelFunc
	metadata  []uuid.UUID
	didCancel bool
}

func (w *rebuildStagingCancelAfterMetadata) AddRebuildMetadata(ctx context.Context, page linkapp.Page, facts linkapp.RebuildMetadata) error {
	if err := w.RebuildWrites.AddRebuildMetadata(ctx, page, facts); err != nil {
		return err
	}
	w.metadata = append(w.metadata, page.ID)
	if page.ID == w.last {
		w.didCancel = true
		w.cancel()
	}
	return nil
}
