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
)

// Both real triggers stay armed in one database. Only the private instance
// limit changes: streaming reaches the early Source link error first, whereas
// buffered reaches Target metadata before any link INSERT. Both must roll back.
func TestRebuildStagingFailurePriority(t *testing.T) {
	f := newRebuildStagingFixture(t, "standard")
	before := ingestCommittedState(t, f.image())
	f.installMetadataTrigger(t)
	f.installPriorityLinkTrigger(t)
	for _, tc := range []struct {
		name, constraint, message, query string
		limit                            uint64
		pages, insertCalls, aliasRows    int
		partial                          rebuildStagingFailureDetail
	}{
		{"streaming", "rebuild_staging_priority_link", "owned rebuild early link", "InsertLinks", 1, 1, 1, 0, rebuildStagingFailureDetail{Heads: 1}},
		{"buffered", "rebuild_staging_metadata_row", "owned rebuild late metadata", "InsertAliases", 0, 2, 0, 1, rebuildStagingFailureDetail{Heads: 2, Tags: 4, Properties: 8, Aliases: 2}},
	} {
		if !t.Run(tc.name, func(t *testing.T) {
			armed := queryStrings(t, f.tm.pool, `SELECT format('%s|%s',tgname,tgenabled) FROM pg_trigger
				WHERE NOT tgisinternal AND ((tgrelid='page_links'::regclass AND tgname='rebuild_staging_priority_fail')
				OR (tgrelid='page_aliases'::regclass AND tgname='rebuild_staging_metadata_fail')) ORDER BY tgname COLLATE "C"`)
			if !slices.Equal(armed, []string{"rebuild_staging_metadata_fail|O", "rebuild_staging_priority_fail|O"}) {
				t.Fatalf("REBUILD_PRIORITY_ARMED: both original triggers must remain enabled: %v", armed)
			}
			listener := f.tm.originListen(t)
			r, observed := f.rebuild(t, tc.limit)
			observed.start()
			_, err := r.Notebook(t.Context(), uuid.MustParse(f.nb))
			work := observed.stop()
			observed.checkTransaction(t, false)
			observed.checkReads(t, f.ids[:tc.pages])
			var cause *pgconn.PgError
			if !errors.As(err, &cause) || cause.Code != "23514" || cause.ConstraintName != tc.constraint || cause.Message != tc.message {
				t.Fatalf("REBUILD_PRIORITY_CAUSE: route=%s error=%v want23514/%s", tc.name, err, tc.constraint)
			}
			var detail rebuildStagingFailureDetail
			if err := json.Unmarshal([]byte(cause.Detail), &detail); err != nil {
				t.Fatal(err)
			}
			want := tc.partial
			want.PID = detail.PID
			if detail.PID == 0 || !observed.pids[detail.PID] || detail != want {
				t.Fatalf("REBUILD_PRIORITY_PARTIAL: route=%s actual=%+v want=%+v pids=%v", tc.name, detail, want, observed.pids)
			}
			failed := 0
			for _, q := range observed.queries {
				if q.err != nil {
					var native *pgconn.PgError
					if q.name != tc.query || q.source != f.sources[tc.pages-1] || !errors.As(q.err, &native) || native != cause { //nolint:errorlint // Require the same server error through the real adapter and TxManager.
						t.Fatalf("REBUILD_PRIORITY_NATIVE: route=%s SQL=%s source=%s error=%v returned=%v", tc.name, q.name, q.source, q.err, err)
					}
					failed++
				} else if failed > 0 && q.name != "rollback" {
					t.Fatalf("REBUILD_PRIORITY_CONTINUED: route=%s SQL after cause=%s", tc.name, q.name)
				}
			}
			if failed != 1 || len(work.failures) != 1 || work.reads.Calls != 0 || work.updates.Calls != 0 || work.upserts != 0 || len(work.notifications) != 0 || work.inserts != (rebuildSingleWriteCount{Calls: int64(tc.insertCalls)}) || work.queries["InsertRebuiltLinks"].Calls != 0 {
				t.Fatalf("REBUILD_PRIORITY_WORK: route=%s failed=%d work=%+v", tc.name, failed, work)
			}
			for _, want := range []struct {
				name        string
				calls, rows int64
			}{
				{"PageContent", int64(tc.pages), int64(tc.pages)},
				{"InsertIndexedPage", int64(tc.pages), int64(tc.partial.Heads)},
				{"InsertTags", int64(tc.pages - tc.insertCalls), int64(tc.partial.Tags)},
				{"InsertProperties", int64(tc.pages - tc.insertCalls), int64(tc.partial.Properties)},
				// The failing second alias INSERT has no completed tag; its
				// first row exists only in the trigger's pre-error DETAIL above.
				{"InsertAliases", int64(tc.pages - tc.insertCalls), int64(tc.aliasRows)},
			} {
				if got := work.queries[want.name]; got != (rebuildSingleWriteCount{Calls: want.calls, Rows: want.rows}) {
					t.Fatalf("REBUILD_PRIORITY_METADATA: route=%s %s=%+v want%d/%d", tc.name, want.name, got, want.calls, want.rows)
				}
			}
			if after := ingestCommittedState(t, f.image()); !slices.Equal(before, after) {
				t.Fatalf("REBUILD_PRIORITY_ROLLBACK: route=%s complete committed image changed:\n%s", tc.name, diff(before, after))
			}
			if notifications := ingestNotifications(t, f.image(), listener); len(notifications) != 0 {
				t.Fatalf("REBUILD_PRIORITY_EVENTS: route=%s committed notifications=%s", tc.name, notifications)
			}
			f.checkLiteralIndex(t)
			checkPages(t, f.tm.pool)
			checkAssets(t, f.tm.pool, f.tm.storage)
			t.Logf("REBUILD_PRIORITY route=%s limit=%d both_armed=%v cause=%s/%s detail=%s committed_events=0", tc.name, tc.limit, armed, cause.Code, cause.ConstraintName, cause.Detail)
		}) {
			// Do not use the shared database for the counter-order until the
			// preceding branch has proved its complete rollback and event window.
			return
		}
	}
}

func (f rebuildStagingFixture) installPriorityLinkTrigger(t *testing.T) {
	t.Helper()
	ingestExec(t, f.tm, fmt.Sprintf(`CREATE FUNCTION rebuild_staging_priority_fail() RETURNS trigger LANGUAGE plpgsql AS $body$
	BEGIN
	 IF NEW.source_id='%s'::uuid AND NEW.range_start=11 AND NEW.target='Target' AND NEW.property_key='' THEN
	  RAISE EXCEPTION 'owned rebuild early link' USING ERRCODE='23514', CONSTRAINT='rebuild_staging_priority_link',
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
	END $body$`, f.sources[0], f.sources[1], f.sources[2]))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), interleavingWait)
		defer cancel()
		_, first := f.tm.pool.Exec(ctx, "DROP TRIGGER IF EXISTS rebuild_staging_priority_fail ON page_links")
		_, second := f.tm.pool.Exec(ctx, "DROP FUNCTION IF EXISTS rebuild_staging_priority_fail()")
		if err := errors.Join(first, second); err != nil {
			t.Errorf("owned rebuild priority trigger cleanup: %v", err)
		}
	})
	ingestExec(t, f.tm, "CREATE TRIGGER rebuild_staging_priority_fail BEFORE INSERT ON page_links FOR EACH ROW EXECUTE FUNCTION rebuild_staging_priority_fail()")
}
