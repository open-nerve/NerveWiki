package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const stableIndexOldFailureBody = "---\nprobe: old\n---\n[[raw/S]] old\n[[raw/T]]\n"
const stableIndexNewFailureBody = "---\nprobe: new\n---\n[[raw/S]] new\n\n[[raw/R]]\n"

// Called before the unchanged real Exec, outside pgx's tracer stack. These
// SELECTs observe completed earlier phases and never supply an SQL answer.
func stableIndexBeforeFailure(ctx context.Context, tx pgx.Tx, u *stableIndexUnit, sql string) {
	name := stableIndexSQLName(sql)
	u.mu.Lock()
	observe := u.failureSQL != "" && name == u.failureSQL && !u.failureProbe
	if observe {
		u.failureProbe = true
	}
	page, cs := u.page, u.cs
	u.mu.Unlock()
	if !observe {
		return
	}
	rows, err := stableWiringRows(ctx, tx, page)
	u.probe(err)
	var content, property string
	var revision, head, extractor int
	var valid, history bool
	err = tx.QueryRow(ctx, `SELECT c.content,c.revision,i.revision,i.extractor,i.frontmatter_valid,
		coalesce((SELECT value::text FROM page_properties WHERE source_id=$1 AND key='probe'),''),
		EXISTS(SELECT 1 FROM page_revisions r JOIN changesets x ON x.id=r.changeset_id
		 WHERE r.node_id=$1 AND r.revision=3 AND r.content=$3 AND r.changeset_id=$2 AND x.kind='edit')
		FROM page_contents c JOIN indexed_pages i ON i.node_id=c.node_id WHERE c.node_id=$1`, page, cs, stableIndexNewFailureBody).
		Scan(&content, &revision, &head, &extractor, &valid, &property, &history)
	u.probe(err)
	u.mu.Lock()
	u.partial = rows
	u.failureBody, u.failureProperty = content, property
	u.failureRevision, u.failureHead = revision, head
	u.failureExtractor, u.failureFrontmatter = extractor, valid
	u.failureContentHistory = history
	u.stageLocked(ctx, "failure_preimage")
	u.mu.Unlock()
}

type stableIndexFailureCase struct{ name, sql, constraint, table, operation string }

func TestStableLinkRetentionRollsBackFailedUnits(t *testing.T) {
	for _, tc := range []stableIndexFailureCase{
		{"missing_link_delete", "DeleteMissingPageLinks", "stable_w1_fixture", "page_links", "DELETE"},
		{"prefilter_write", "WritePageLinksRetainingEqualPrefiltered", "stable_w7_fixture", "page_links", "INSERT"},
		{"side_properties", "InsertProperties", "stable_properties_fixture", "page_properties", "INSERT"},
		{"set", "SetResolutions", "stable_set_fixture", "page_links", "UPDATE"},
		{"publisher", "", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) { testStableIndexFailedUnit(t, tc) })
	}
}

func stableIndexInstallTrigger(t *testing.T, tm acmeTeam, tc stableIndexFailureCase, p, r uuid.UUID) func() error {
	t.Helper()
	if tc.name == "publisher" {
		return func() error { return nil }
	}
	var predicate string
	returnRow := "NEW"
	switch tc.name {
	case "missing_link_delete":
		predicate = fmt.Sprintf("OLD.source_id='%s'::uuid AND OLD.range_start=35", p)
		returnRow = "OLD"
	case "prefilter_write":
		predicate = fmt.Sprintf("NEW.source_id='%s'::uuid AND NEW.range_start=36 AND NEW.target='raw/R'", p)
	case "side_properties":
		predicate = fmt.Sprintf(`NEW.source_id='%s'::uuid AND NEW.key='probe' AND NEW.value='"new"'::jsonb`, p)
	case "set":
		predicate = fmt.Sprintf("NEW.source_id='%s'::uuid AND NEW.range_start=36 AND OLD.resolved_id IS NULL AND NEW.resolved_id='%s'::uuid", p, r)
	default:
		t.Fatalf("unrecognized owned trigger case %s", tc.name)
	}
	// Only literal test identifiers and already parsed UUIDs enter this DDL.
	// Every leaf owns its pgtest database; no global or client configuration.
	ingestExec(t, tm, fmt.Sprintf(`CREATE FUNCTION stable_index_fail() RETURNS trigger LANGUAGE plpgsql AS $body$
	BEGIN
	 IF %s THEN RAISE EXCEPTION 'stable Index owned trigger' USING ERRCODE='23514', CONSTRAINT='%s'; END IF;
	 RETURN %s;
	END $body$`, predicate, tc.constraint, returnRow))
	var once sync.Once
	var dropErr error
	drop := func() error {
		once.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), interleavingWait)
			defer cancel()
			_, first := tm.pool.Exec(ctx, "DROP TRIGGER IF EXISTS stable_index_fail ON "+tc.table)
			_, second := tm.pool.Exec(ctx, "DROP FUNCTION IF EXISTS stable_index_fail()")
			dropErr = errors.Join(first, second)
		})
		return dropErr
	}
	t.Cleanup(func() {
		if err := drop(); err != nil {
			t.Errorf("owned stable trigger cleanup: %v", err)
		}
	})
	ingestExec(t, tm, "CREATE TRIGGER stable_index_fail BEFORE "+tc.operation+" ON "+tc.table+" FOR EACH ROW EXECUTE FUNCTION stable_index_fail()")
	return drop
}

func testStableIndexFailedUnit(t *testing.T, tc stableIndexFailureCase) {
	control := &stableIndexControl{}
	w := stableIndexWire(t, wiring{}, control)
	f := newIngestFixture(t, &w)
	r := f.tm.createPageWith(t, "alice", f.nb, f.raw, "R", "Independent R evidence.\n")
	marker := f.tm.createPage(t, "alice", f.nb, "", "Stable marker")
	f.write(t, f.p, stableIndexOldFailureBody)
	P, N, S, T, R := uuid.MustParse(f.p), uuid.MustParse(f.nb), uuid.MustParse(f.s), uuid.MustParse(f.t), uuid.MustParse(r)
	stableIndexDistinct(t, P, N, S, T, R)
	old := []stableWiringRow{
		stableIndexRow(P, N, 21, 26, "wikilink", "raw/S", "s", S, false, false),
		stableIndexRow(P, N, 35, 40, "wikilink", "raw/T", "t", T, false, false),
	}
	replaced := []stableWiringRow{
		stableIndexRow(P, N, 21, 26, "wikilink", "raw/S", "s", S, false, false),
		stableIndexRow(P, N, 36, 41, "wikilink", "raw/R", "r", uuid.Nil(), false, false),
	}
	final := []stableWiringRow{
		stableIndexRow(P, N, 21, 26, "wikilink", "raw/S", "s", S, false, false),
		stableIndexRow(P, N, 36, 41, "wikilink", "raw/R", "r", R, false, false),
	}
	stableIndexInitial(t, f.tm, P, old)
	f.tm.settle(t, f.nb, marker)
	imageFixture := ingestInterleaveFixture{tm: f.tm, nb: f.nb}
	before := ingestCommittedState(t, imageFixture)
	listener := f.tm.originListen(t)
	drop := stableIndexInstallTrigger(t, f.tm, tc, P, R)
	u := newStableIndexUnit(f.p, f.nb, 3, stableIndexNewFailureBody)
	u.failureSQL = tc.sql
	if tc.name == "publisher" {
		u.publisherCause = errors.New("after real stable links publication")
	}
	control.arm(u)
	control.mu.Lock()
	control.txUnit = u
	control.mu.Unlock()
	// No fixture GET or other write occurs between arming and this exact PUT.
	reply := f.tm.sender(t, contentWrite("alice", f.p, stableIndexNewFailureBody, 2, ""))()
	select {
	case <-u.managerDone:
	case <-time.After(interleavingWait):
		t.Fatal("stable manager did not return; deadline is not a native failure oracle")
	}
	control.arm()
	if reply.err != nil || reply.status != http.StatusInternalServerError {
		t.Fatalf("stable failed unit response=%d %s err=%v", reply.status, reply.body, reply.err)
	}
	stableIndexCheckFailure(t, u, tc, old, replaced, final, R)
	if after := ingestCommittedState(t, imageFixture); !slices.Equal(after, before) {
		t.Fatalf("stable complete rollback changed rows:\nbefore=%v\nafter=%v", before, after)
	}
	if count(t, f.tm.pool, `SELECT count(*) FROM changesets WHERE id=$1`, u.cs) != 0 || count(t, f.tm.pool, `SELECT count(*) FROM changeset_sources WHERE changeset_id=$1`, u.cs) != 0 {
		t.Fatal("stable failed unit leaked its changeset or marks")
	}
	if got := ingestNotifications(t, imageFixture, listener); len(got) != 0 {
		t.Fatalf("stable rollback delivered committed notifications=%s", got)
	}
	ctx, cancel := context.WithTimeout(t.Context(), interleavingWait)
	conn, err := control.pool.Acquire(ctx)
	if err != nil {
		cancel()
		t.Fatalf("stable original app pool recovery: %v", err)
	}
	var one int
	err = conn.QueryRow(ctx, "SELECT 1").Scan(&one)
	conn.Release()
	cancel()
	if err != nil || one != 1 {
		t.Fatalf("stable app pool recovered result=%d err=%v", one, err)
	}
	if err := drop(); err != nil {
		t.Fatal(err)
	}
	// A real successful retry must still start at r2. It is this leaf's healthy
	// control, with exactly the same Writer and original Index dependencies.
	stream := openStream(t, f.tm.base, "alice", f.tm.tokens["alice"])
	retry := newStableIndexUnit(f.p, f.nb, 3, stableIndexNewFailureBody)
	control.arm(retry)
	f.tm.send(t, contentWrite("alice", f.p, stableIndexNewFailureBody, 2, ""), http.StatusOK)
	control.arm()
	stableIndexCheck(t, f.tm, retry, stableIndexWant{before: old, replaced: replaced, final: final, changed: stableIndexLiteralLinks(final[1:]), dropped: stableIndexIDs(S, T), keys: []string{"r", "s"}})
	cs := ingestCS(t, f.tm, f.p, 3)
	assertIngestMarks(t, f.tm, cs, r)
	stableIndexEvents(t, f.tm, f.nb, marker, stream, false, 3, f.p, f.s, f.t, r)
	checkPages(t, f.tm.pool)
	checkLinks(t, f.tm.pool)
	f.tm.checkRebuilt(t, f.nb)
}

func stableIndexCheckFailure(t *testing.T, u *stableIndexUnit, tc stableIndexFailureCase, old, replaced, final []stableWiringRow, target uuid.UUID) {
	t.Helper()
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.units != 1 || u.cs == uuid.Nil() || u.replaceCalls != 1 || len(u.probeFailures) != 0 || u.writerBody != u.body || u.writerRevision != 3 || u.beforeHead != 2 {
		t.Fatalf("stable failure did not enter the exact original unit: units=%d cs=%s replaces=%d body=%q rev=%d oldhead=%d probes=%v", u.units, u.cs, u.replaceCalls, u.writerBody, u.writerRevision, u.beforeHead, u.probeFailures)
	}
	if !reflect.DeepEqual(u.before, old) || !reflect.DeepEqual(u.incoming, stableIndexLiteralFacts(replaced)) {
		t.Fatalf("stable failure initial/incoming facts=%+v/%+v", u.before, u.incoming)
	}
	stableIndexCheckInspect(t, u, len(old))
	if u.inspectCalls != 1 || !u.inspectAllowed || !slices.Equal(u.inspectTargets, stableIndexIDs(*old[0].Resolved, *old[1].Resolved)) {
		t.Fatalf("stable failure actual Inspect: calls=%d allowed=%t old D=%v", u.inspectCalls, u.inspectAllowed, u.inspectTargets)
	}
	if u.cause == nil || !errors.Is(u.callbackErr, u.cause) || !errors.Is(u.managerErr, u.cause) || errors.Is(u.managerErr, pgx.ErrTxCommitRollback) {
		t.Fatalf("stable original cause lost: cause=%v callback=%v manager=%v", u.cause, u.callbackErr, u.managerErr)
	}
	failureOrder := 0
	counts := map[string]int{}
	pid := uint32(0)
	for _, stage := range u.stages {
		if stage.Name == "observer" {
			pid = stage.PID
		}
	}
	if pid == 0 {
		t.Fatal("stable failure has no original observer PID")
	}
	for _, q := range u.queries {
		counts[q.Name]++
		if q.PID != pid || !q.Done {
			t.Errorf("stable failure SQL lost original PID/completion: %+v", q)
		}
		if q.Err != nil && q.Name != tc.sql {
			t.Errorf("stable unexpected native SQL error before or after selected cause: %+v", q)
		}
		if q.Name == tc.sql && q.Err != nil {
			if failureOrder != 0 || !errors.Is(q.Err, u.cause) || q.ContextErr != nil {
				t.Errorf("stable native error identity/phase: %+v", q)
			}
			failureOrder = q.End
		}
	}
	if tc.name == "publisher" {
		failureOrder = stableIndexStage(u, "publisher_error")
		//nolint:errorlint // The boundary must preserve the original publisher error object.
		if u.cause != u.publisherCause || !reflect.DeepEqual(u.replaced, replaced) || !reflect.DeepEqual(u.afterSet, final) || u.setCalls != 1 || u.markCalls != 1 || u.publishCalls != 1 || !slices.Equal(u.markIDs, []uuid.UUID{target}) || u.markResult.Added != 1 || u.markResult.Count != 1 || u.storedMarks != 1 {
			t.Fatalf("stable actual Set/Add/publish did not precede boundary failure: replaced=%+v final=%+v Set=%d marks=%d ids=%v result=%+v stored=%d publishers=%d cause=%v", u.replaced, u.afterSet, u.setCalls, u.markCalls, u.markIDs, u.markResult, u.storedMarks, u.publishCalls, u.cause)
		}
		if !slices.Equal(u.dropped.Targets, stableIndexIDs(*old[0].Resolved, *old[1].Resolved)) || len(u.dropped.AliasKeys) != 0 || u.published.WorkspaceID == uuid.Nil() || u.published.NotebookID != u.notebook || u.published.Pages == nil || len(u.published.Pages) != 0 || u.published.Aliases || !slices.Equal(stableIndexIDs(u.published.Targets...), stableIndexIDs(*old[0].Resolved, *old[1].Resolved, target)) {
			t.Errorf("stable actual pre-rollback complete D/event: D=%+v event=%+v", u.dropped, u.published)
		}
		notifies := 0
		for _, q := range u.queries {
			if strings.Contains(q.SQL, "pg_notify") && q.Start > stableIndexStage(u, "publish") && q.End < stableIndexStage(u, "published") && q.Err == nil {
				notifies++
			}
		}
		if notifies != 1 || stableIndexStage(u, "set_finished") >= stableIndexStage(u, "marks") || stableIndexStage(u, "marks_finished") >= stableIndexStage(u, "publish") || stableIndexStage(u, "published") >= failureOrder {
			t.Fatalf("stable real publication not proved before sentinel: notifies=%d stages=%+v", notifies, u.stages)
		}
	} else {
		var pg *pgconn.PgError
		if !errors.As(u.cause, &pg) || pg.Code != "23514" || pg.ConstraintName != tc.constraint || failureOrder == 0 {
			t.Fatalf("stable native %s cause=%v, want23514/%s", tc.sql, u.cause, tc.constraint)
		}
		if !u.failureProbe || u.failureBody != stableIndexNewFailureBody || u.failureRevision != 3 || !u.failureContentHistory || u.failureExtractor != 2 || !u.failureFrontmatter {
			t.Fatalf("stable native failure lacks real prewritten content/history: body=%q revision=%d history=%t extractor=%d frontmatter=%t", u.failureBody, u.failureRevision, u.failureContentHistory, u.failureExtractor, u.failureFrontmatter)
		}
		wantPartial, wantHead, wantProperty := replaced, 3, ""
		switch tc.name {
		case "missing_link_delete":
			wantPartial, wantHead, wantProperty = old, 2, `"old"`
		case "prefilter_write":
			wantPartial = old[:1]
		case "set":
			wantProperty = `"new"`
		}
		if !reflect.DeepEqual(u.partial, wantPartial) || u.failureHead != wantHead || u.failureProperty != wantProperty {
			t.Fatalf("stable actual pre-%s partial rows=%+v head=%d property=%q want rows=%+v head=%d property=%q", tc.sql, u.partial, u.failureHead, u.failureProperty, wantPartial, wantHead, wantProperty)
		}
		if u.markCalls != 0 || u.publishCalls != 0 {
			t.Errorf("stable native failure continued marks/publish: %d/%d", u.markCalls, u.publishCalls)
		}
	}
	if failureOrder == 0 {
		t.Fatal("stable first actual failure not observed")
	}
	// Each leaf exercises one real statement after the complete successful
	// prefix. No later phase, including empty side-fact writes, may be tried.
	phases := []string{"DeleteMissingPageLinks", "DeleteAliasesOf", "DeleteTagsOf", "DeletePropertiesOf", "DeleteIndexedPages", "InsertIndexedPage", "WritePageLinksRetainingEqualPrefiltered", "InsertTags", "InsertProperties", "InsertAliases", "SetResolutions"}
	last := len(phases) - 1
	if tc.sql != "" {
		last = slices.Index(phases, tc.sql)
	}
	previous := 0
	for _, q := range u.queries {
		if q.Name == "InspectPageLinksForRetention" {
			previous = q.End
		}
	}
	for i, name := range phases {
		wantCalls := 0
		if i <= last {
			wantCalls = 1
		}
		if counts[name] != wantCalls {
			t.Errorf("stable actual phase %s calls=%d want%d", name, counts[name], wantCalls)
		}
		if wantCalls == 1 {
			for _, q := range u.queries {
				if q.Name == name {
					if q.Start <= previous || q.End > failureOrder {
						t.Errorf("stable actual prefix phase order: %+v previous=%d failure=%d", q, previous, failureOrder)
					}
					previous = q.End
				}
			}
		}
	}
	for _, q := range u.queries {
		if q.Start > failureOrder && q.Name != "rollback" {
			t.Errorf("stable SQL attempted after original failure: %+v", q)
		}
		if strings.HasPrefix(q.Name, "savepoint") || strings.HasPrefix(q.Name, "release savepoint") || strings.HasPrefix(q.Name, "rollback to") {
			t.Errorf("stable failure unexpectedly nested/retried: %+v", q)
		}
	}
	for _, s := range u.stages {
		if s.Order > failureOrder && s.Name != "callback_exit" && s.Name != "manager_exit" {
			t.Errorf("stable business stage attempted after failure: %+v", s)
		}
	}
	if counts["begin"] != 1 || counts["rollback"] != 1 || counts["commit"] != 0 || counts["InspectPageLinksForRetention"] != 1 || counts["DeleteLinksOf"] != 0 || counts["InsertLinks"] != 0 || counts["WritePageLinksRetainingEqual"] != 0 {
		t.Errorf("stable original end or branch/fallback calls=%v", counts)
	}
	for _, q := range u.queries {
		if q.Name == "rollback" && (q.Err != nil || q.ContextErr != nil || !q.Bounded || q.Start <= stableIndexStage(u, "callback_exit") || q.End >= stableIndexStage(u, "manager_exit")) {
			t.Errorf("stable original bounded rollback/end order=%+v stages=%+v", q, u.stages)
		}
	}
	// The actual named statement must be the first native failing statement.
	if tc.sql != "" && counts[tc.sql] != 1 {
		t.Errorf("stable actual failing statement %s count=%d", tc.sql, counts[tc.sql])
	}
	t.Logf("STABLE_FAILURE leaf=%s pid=%d actual_cause=%v end_counts=%v", tc.name, pid, u.cause, counts)
}
