package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events"
	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	linkapp "github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The lower-only instance limit exercises the real fallback, with the same
// API-written notebook and real parser/transaction/Store as the normal route.
// LLM's two original link-free template pages precede the three nonempty pages.
func TestRebuildStagingFallbackKeepsWholeIndex(t *testing.T) {
	for _, mode := range []string{"standard", "llm_wiki"} {
		t.Run(mode, func(t *testing.T) {
			f := newRebuildStagingFixture(t, mode)
			before := ingestCommittedState(t, f.image())
			listener := f.tm.originListen(t)
			for _, tc := range []struct {
				name string
				at   int
			}{{"normal", -1}, {"first", 0}, {"middle", 1}, {"last", 2}} {
				t.Run(tc.name, func(t *testing.T) {
					limit := uint64(0)
					if tc.at >= 0 {
						limit = f.beforePageLimit(tc.at + 1)
					}
					r, observed := f.rebuild(t, limit)
					observed.start()
					got, err := r.Notebook(t.Context(), uuid.MustParse(f.nb))
					work := observed.stop()
					if err != nil {
						t.Fatal(err)
					}
					if got.Pages != len(f.ids) || got.Links != 21 || got.Unresolved != 3 || len(got.Clashes) != 0 {
						t.Fatalf("REBUILD_STAGING_COUNTS: got %+v; want pages=%d links=21 unresolved=3", got, len(f.ids))
					}
					f.checkLiteralIndex(t)
					if after := ingestCommittedState(t, f.image()); !slices.Equal(before, after) {
						t.Fatalf("REBUILD_STAGING_IMAGE: complete normally-written image changed:\n%s", diff(before, after))
					}
					observed.checkReads(t, f.ids)
					f.checkWork(t, observed, work, tc.at)
					published := ingestNotifications(t, f.image(), listener)
					if len(published) != 1 || len(work.notifications) != 1 || string(published[0]) != work.notifications[0] {
						t.Fatalf("REBUILD_STAGING_EVENT: committed=%s actual pg_notify=%v", published, work.notifications)
					}
					checkPages(t, f.tm.pool)
					checkAssets(t, f.tm.pool, f.tm.storage)
					t.Logf("REBUILD_STAGING_NATIVE mode=%s boundary=%s limit=%d rowsize=%d pid=%v work=%+v", mode, tc.name, limit, reflect.TypeFor[domain.Link]().Size(), observed.pids, work.queries)
				})
			}
		})
	}
}

// The server raises a real constraint error at the seventh row of the second
// page. Its DETAIL proves the earlier page and six rows of this statement were
// already visible inside the same transaction, before its complete rollback.
func TestRebuildStagingLateInsertFailureRollsBack(t *testing.T) {
	for _, phase := range []string{"drain", "resolved_insert"} {
		t.Run(phase, func(t *testing.T) {
			f := newRebuildStagingFixture(t, "standard")
			before := ingestCommittedState(t, f.image())
			listener := f.tm.originListen(t)
			f.installLateTrigger(t)
			limit := uint64(0)
			if phase == "drain" {
				limit = f.beforePageLimit(3)
			}
			r, observed := f.rebuild(t, limit)
			observed.start()
			_, err := r.Notebook(t.Context(), uuid.MustParse(f.nb))
			work := observed.stop()
			var cause *pgconn.PgError
			if !errors.As(err, &cause) || cause.Code != "23514" || cause.ConstraintName != "rebuild_staging_late_row" || cause.Message != "owned rebuild late INSERT" {
				t.Fatalf("REBUILD_STAGING_NATIVE_CAUSE: got %v; want original 23514/rebuild_staging_late_row", err)
			}
			var detail struct {
				PID                              uint32
				Prior, Current, Resolved         int
				Heads, Tags, Properties, Aliases int
				Future                           int
			}
			if err := json.Unmarshal([]byte(cause.Detail), &detail); err != nil {
				t.Fatal(err)
			}
			wantHeads, wantTags, wantProperties, wantAliases, wantFuture, wantResolved := 3, 6, 12, 6, 1, 6
			if phase == "drain" {
				wantHeads, wantTags, wantProperties, wantAliases, wantFuture, wantResolved = 2, 4, 8, 3, 0, 0
			}
			if detail.Prior != 7 || detail.Current != 6 || detail.Resolved != wantResolved || detail.Heads != wantHeads || detail.Tags != wantTags || detail.Properties != wantProperties || detail.Aliases != wantAliases || detail.Future != wantFuture {
				t.Fatalf("REBUILD_STAGING_LATE_ROW: real partial image=%+v; want prior=7 current=6 resolved=%d metadata=%d/%d/%d/%d future=%d", detail, wantResolved, wantHeads, wantTags, wantProperties, wantAliases, wantFuture)
			}
			observed.checkReads(t, f.ids)
			observed.checkTransaction(t, false)
			if !observed.pids[detail.PID] || detail.PID == 0 {
				t.Fatalf("REBUILD_STAGING_TX: trigger pid=%d query pids=%v", detail.PID, observed.pids)
			}
			failed := 0
			for _, q := range observed.queries {
				if q.err != nil {
					var native *pgconn.PgError
					if !q.insert || !errors.As(q.err, &native) || native != cause { //nolint:errorlint // Require the exact native PgError object through the adapter/manager.
						t.Fatalf("REBUILD_STAGING_NATIVE_CAUSE: native error=%v returned=%v", q.err, err)
					}
					if (phase == "drain") != (q.name == "InsertLinks") {
						t.Fatalf("REBUILD_STAGING_FAILURE_PHASE: phase=%s actual SQL=%s", phase, q.name)
					}
					failed++
				} else if failed > 0 && q.name != "rollback" {
					t.Fatalf("REBUILD_STAGING_FAILURE_CONTINUED: SQL after native cause=%s", q.name)
				}
			}
			if failed != 1 || len(work.failures) != 1 || len(work.notifications) != 0 || work.reads.Calls != 0 || work.updates.Calls != 0 {
				t.Fatalf("REBUILD_STAGING_FAILURE_CONTINUED: failed=%d actual work=%+v", failed, work)
			}
			if after := ingestCommittedState(t, f.image()); !slices.Equal(before, after) {
				t.Fatalf("REBUILD_STAGING_ROLLBACK: full committed image differs:\n%s", diff(before, after))
			}
			if got := ingestNotifications(t, f.image(), listener); len(got) != 0 {
				t.Fatalf("REBUILD_STAGING_ROLLBACK_EVENT: committed notifications=%s", got)
			}
			checkPages(t, f.tm.pool)
			checkAssets(t, f.tm.pool, f.tm.storage)
			t.Logf("REBUILD_STAGING_NATIVE_FAILURE phase=%s detail=%s cause=%s/%s", phase, cause.Detail, cause.Code, cause.ConstraintName)
		})
	}
}

type rebuildStagingFixture struct {
	tm      acmeTeam
	nb      string
	asset   string
	ids     []uuid.UUID
	sources [3]uuid.UUID
}

func newRebuildStagingFixture(t *testing.T, mode string) rebuildStagingFixture {
	t.Helper()
	tm := newAcmeTeam(t, "", "")
	nb := tm.createWith(t, tm.tokens["alice"], fmt.Sprintf(`{"name":"Staged rebuild","mode":%q}`, mode))
	f := rebuildStagingFixture{tm: tm, nb: nb.ID}
	const prefix = "---\n\"\": '[[Target]]'\nstatus: ready\ntags: [Alpha]\n"
	const body = "---\n[[Target.md#Part|shown]] [[Late]] [[Twin]] [](asset.txt) [[Missing]]\n#Tag #Tag\n"
	for i, p := range []struct{ name, aliases string }{
		{"Source", "aliases: ['[[Target]]']\n"},
		{"Target", "aliases: ['[[Target]]', Twin]\n"},
		{"AliasA", "aliases: ['[[Target]]', Late, Twin]\n"},
	} {
		f.sources[i] = uuid.MustParse(tm.createPageWith(t, "alice", f.nb, "", p.name, prefix+p.aliases+body))
	}
	f.asset = tm.upload(t, "alice", f.nb, "", "asset.txt", "actual asset bytes\n").ID
	targets := linkTargets{page.NewLinkTargets(tm.pool)}
	var err error
	f.ids, err = targets.PageIDs(t.Context(), uuid.MustParse(f.nb))
	if err != nil {
		t.Fatal(err)
	}
	wantPages := 3
	if mode == "llm_wiki" {
		wantPages += 2
	}
	if len(f.ids) != wantPages || !slices.Equal(f.ids[len(f.ids)-3:], f.sources[:]) {
		t.Fatalf("fixture page order=%v; want only optional templates then %v", f.ids, f.sources)
	}
	f.checkLiteralIndex(t)
	return f
}

func (f rebuildStagingFixture) image() ingestInterleaveFixture {
	return ingestInterleaveFixture{tm: f.tm, nb: f.nb}
}

func (f rebuildStagingFixture) beforePageLimit(n int) uint64 {
	// Resource-policy arithmetic for these seven literal rows, not a copy of
	// the product helper: five short Targets cost16 each, asset.txt and Target.md
	// cost18 each (116 total). The aliases.0
	// property costs18 and Part/shown16 each, hence166 string bytes in total.
	// The seven Targets are Target,Target,Target.md,Late,Twin,asset.txt,Missing.
	stringsCharge := uint64(166)
	pageCharge := 7*uint64(reflect.TypeFor[domain.Link]().Size()) + stringsCharge
	slots := uint64(len(f.ids)) * uint64(reflect.TypeFor[[]domain.Link]().Size()+reflect.TypeFor[int]().Size())
	return slots + uint64(n)*pageCharge - 1
}

func (f rebuildStagingFixture) rebuild(t *testing.T, limit uint64) (linkapp.Rebuild, *rebuildStagingObserve) {
	t.Helper()
	pc := f.tm.pool.Config()
	o := &rebuildStagingObserve{single: &rebuildSingleWriteTrace{previous: pc.ConnConfig.Tracer}, ids: f.ids}
	pc.ConnConfig.Tracer = o
	pool, err := pgxpool.NewWithConfig(t.Context(), pc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(t.Context()); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t, f.tm.url, false)
	md, budget, err := parsing(cfg, slog.New(slog.DiscardHandler), pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	targets := linkTargets{page.NewLinkTargets(pool)}
	store := postgresadapter.New(pool)
	return linkapp.Rebuild{
		Tx:        postgres.NewTxManager(pool, cfg.Database.CommitTimeout),
		Index:     linkapp.Index{Store: store, Pages: targets, Publisher: linkEvents{events.NewPublisher()}},
		Contents:  rebuildStagingContents{Contents: targets, observed: o},
		Notebooks: notebook.NewNotebooks(pool),
		Parser:    rebuildStagingParser{Parser: markdownadapter.NewParser(md, budget), observed: o},
		Writes:    store, StagingLimit: limit,
	}, o
}

func (f rebuildStagingFixture) checkLiteralIndex(t *testing.T) {
	t.Helper()
	positions := [3][7][2]int{
		{{11, 17}, {62, 68}, {79, 88}, {104, 108}, {113, 117}, {123, 132}, {136, 143}},
		{{11, 17}, {62, 68}, {85, 94}, {110, 114}, {119, 123}, {129, 138}, {142, 149}},
		{{11, 17}, {62, 68}, {91, 100}, {116, 120}, {125, 129}, {135, 144}, {148, 155}},
	}
	for i, source := range f.sources {
		// JSON preserves every SQL NULL, including body vs the empty property
		// and NULL vs a nonempty alternative key; no printable sentinel is used.
		got := queryStrings(t, f.tm.pool, `SELECT json_build_array(source_id,notebook_id,range_start,range_end,
			kind,property_key,target,anchor,display,target_key,target_alt_key,resolved_id,ambiguous,resolved_asset,aliases)::text
			FROM page_links WHERE source_id=$1 ORDER BY range_start`, source)
		values := [][]any{
			{"wikilink", "", "Target", nil, nil, "target", nil, f.sources[1].String(), false, false, false},
			{"wikilink", "aliases.0", "Target", nil, nil, "target", nil, f.sources[1].String(), false, false, true},
			{"wikilink", nil, "Target.md", "Part", "shown", "target", "target.md", f.sources[1].String(), false, false, false},
			{"wikilink", nil, "Late", nil, nil, "late", nil, f.sources[2].String(), false, false, false},
			{"wikilink", nil, "Twin", nil, nil, "twin", nil, f.sources[1].String(), true, false, false},
			{"link", nil, "asset.txt", nil, nil, "asset.txt", nil, f.asset, false, true, false},
			{"wikilink", nil, "Missing", nil, nil, "missing", nil, nil, false, false, false},
		}
		var want []string
		for j, fields := range values {
			row := append([]any{source.String(), f.nb, positions[i][j][0], positions[i][j][1]}, fields...)
			encoded, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			want = append(want, string(encoded))
		}
		for j, row := range got {
			var compact bytes.Buffer
			if err := json.Compact(&compact, []byte(row)); err != nil {
				t.Fatal(err)
			}
			got[j] = compact.String()
		}
		if !slices.Equal(got, want) {
			t.Fatalf("REBUILD_STAGING_LITERAL: source=%s\n%v\nwant\n%v", source, got, want)
		}
	}
	if n := count(t, f.tm.pool, "SELECT count(*) FROM page_links WHERE notebook_id=$1", f.nb); n != 21 {
		t.Fatalf("REBUILD_STAGING_COMPLETE: links=%d want21", n)
	}
	heads := queryStrings(t, f.tm.pool, `SELECT format('%s|%s|%s|%s',n.name,i.revision,i.extractor,i.frontmatter_valid::text)
		FROM indexed_pages i JOIN nodes n ON n.id=i.node_id WHERE i.notebook_id=$1 ORDER BY n.name COLLATE "C"`, f.nb)
	wantHeads := []string{"AliasA|1|2|true", "Source|1|2|true", "Target|1|2|true"}
	if len(f.ids) == 5 {
		wantHeads = append(wantHeads, "AGENTS|1|2|true", "raw|1|2|true")
		slices.Sort(wantHeads)
	}
	if !slices.Equal(heads, wantHeads) {
		t.Fatalf("REBUILD_STAGING_HEADS: got%v want%v", heads, wantHeads)
	}
	gotSides := queryStrings(t, f.tm.pool, `SELECT line FROM (
		SELECT format('tag|%s|%s|%s|%s',n.name,x.tag_key,x.tag,x.count) AS line FROM page_tags x JOIN nodes n ON n.id=x.source_id WHERE x.notebook_id=$1
		UNION ALL SELECT format('property|%s|%s|%s|%s',n.name,x.position,x.key,x.value::text) FROM page_properties x JOIN nodes n ON n.id=x.source_id WHERE x.notebook_id=$1
		UNION ALL SELECT format('alias|%s|%s|%s',n.name,x.alias_key,x.alias) FROM page_aliases x JOIN nodes n ON n.id=x.source_id WHERE x.notebook_id=$1
	) facts ORDER BY line COLLATE "C"`, f.nb)
	var wantSides []string
	for _, name := range []string{"Source", "Target", "AliasA"} {
		wantSides = append(wantSides, "tag|"+name+"|alpha|Alpha|1", "tag|"+name+"|tag|Tag|2",
			`property|`+name+`|0||"[[Target]]"`, `property|`+name+`|1|status|"ready"`, `property|`+name+`|2|tags|["Alpha"]`,
			"alias|"+name+"|[[target]]|[[Target]]")
	}
	wantSides = append(wantSides, `property|Source|3|aliases|["[[Target]]"]`, `property|Target|3|aliases|["[[Target]]", "Twin"]`,
		`property|AliasA|3|aliases|["[[Target]]", "Late", "Twin"]`, "alias|Target|twin|Twin", "alias|AliasA|late|Late", "alias|AliasA|twin|Twin")
	slices.Sort(wantSides)
	if !slices.Equal(gotSides, wantSides) {
		t.Fatalf("REBUILD_STAGING_SIDES: got%v want%v", gotSides, wantSides)
	}
}

func (f rebuildStagingFixture) checkWork(t *testing.T, o *rebuildStagingObserve, w rebuildSingleWriteWork, fallbackAt int) {
	t.Helper()
	o.checkTransaction(t, true)
	if len(w.failures) != 0 || w.upserts != 0 || w.inserts.Calls != int64(len(f.ids)) || w.inserts.Rows != 21 {
		t.Fatalf("REBUILD_STAGING_WRITES: %+v", w)
	}
	if got := w.queries["PageContent"]; got != (rebuildSingleWriteCount{Calls: int64(len(f.ids)), Rows: int64(len(f.ids))}) {
		t.Fatalf("REBUILD_STAGING_NATIVE_CONTENT: got%+v want one real row per page", got)
	}
	for _, x := range []struct {
		name string
		rows int64
	}{{"InsertIndexedPage", int64(len(f.ids))}, {"InsertTags", 6}, {"InsertProperties", 12}, {"InsertAliases", 6}} {
		if got := w.queries[x.name]; got.Calls != int64(len(f.ids)) || got.Rows != x.rows {
			t.Fatalf("REBUILD_STAGING_METADATA_ONCE: %s=%+v; want calls=%d rows=%d", x.name, got, len(f.ids), x.rows)
		}
	}
	var gotOrder, wantOrder []string
	for _, q := range o.queries {
		if q.name == "InsertIndexedPage" || q.insert {
			kind := "head:"
			if q.insert {
				kind = "links:"
				if (fallbackAt >= 0) != (q.name == "InsertLinks") {
					t.Fatalf("REBUILD_STAGING_ROUTE: fallback=%d link SQL=%s", fallbackAt, q.name)
				}
			}
			gotOrder = append(gotOrder, kind+q.source.String())
		}
	}
	boundary := len(f.ids)
	if fallbackAt >= 0 {
		boundary = slices.Index(f.ids, f.sources[fallbackAt])
	}
	for _, id := range f.ids[:boundary] {
		wantOrder = append(wantOrder, "head:"+id.String())
	}
	for _, id := range f.ids[:boundary] {
		wantOrder = append(wantOrder, "links:"+id.String())
	}
	for _, id := range f.ids[boundary:] {
		wantOrder = append(wantOrder, "head:"+id.String(), "links:"+id.String())
	}
	if !slices.Equal(gotOrder, wantOrder) {
		t.Fatalf("REBUILD_STAGING_DRAIN_ORDER: got%v want%v", gotOrder, wantOrder)
	}
	if fallbackAt >= 0 {
		if w.reads != (rebuildSingleWriteCount{Calls: 1, Rows: 21}) || w.updates != (rebuildSingleWriteCount{Calls: 1, Rows: 18}) || w.queries["LinksReached"] != w.reads || w.queries["SetResolutions"] != w.updates {
			t.Fatalf("REBUILD_STAGING_FULL_FALLBACK: want complete21 read/18 update; got%+v", w)
		}
	} else if w.reads.Calls != 0 || w.updates.Calls != 0 {
		t.Fatalf("REBUILD_STAGING_NORMAL: want no readback/update; got%+v", w)
	}
	if w.queries["LinkTargetsByKeys"].Calls == 0 || w.queries["AliasesByKeys"].Calls == 0 {
		t.Fatalf("REBUILD_STAGING_FRESH: actual fresh target/alias queries absent: %+v", w.queries)
	}
}

func (f rebuildStagingFixture) installLateTrigger(t *testing.T) {
	t.Helper()
	// UUIDs come from the actual API fixture; identifiers are fixed and each
	// test owns its pgtest database. No synthetic result or application error.
	ingestExec(t, f.tm, fmt.Sprintf(`CREATE FUNCTION rebuild_staging_fail() RETURNS trigger LANGUAGE plpgsql AS $body$
	BEGIN
	 IF NEW.source_id='%s'::uuid AND NEW.range_start=142 AND NEW.target='Missing' THEN
	  RAISE EXCEPTION 'owned rebuild late INSERT' USING ERRCODE='23514', CONSTRAINT='rebuild_staging_late_row',
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
		_, first := f.tm.pool.Exec(ctx, "DROP TRIGGER IF EXISTS rebuild_staging_fail ON page_links")
		_, second := f.tm.pool.Exec(ctx, "DROP FUNCTION IF EXISTS rebuild_staging_fail()")
		if err := errors.Join(first, second); err != nil {
			t.Errorf("owned rebuild trigger cleanup: %v", err)
		}
	})
	ingestExec(t, f.tm, "CREATE TRIGGER rebuild_staging_fail BEFORE INSERT ON page_links FOR EACH ROW EXECUTE FUNCTION rebuild_staging_fail()")
}

type rebuildStagingQuery struct {
	name   string
	source uuid.UUID
	insert bool
	err    error
}
type rebuildStagingObserve struct {
	single          *rebuildSingleWriteTrace
	ids             []uuid.UUID
	mu              sync.Mutex
	active          bool
	pids            map[uint32]bool
	queries         []rebuildStagingQuery
	content, parsed map[uuid.UUID]int
	current         uuid.UUID
}
type rebuildStagingQueryKey struct{}

func (o *rebuildStagingObserve) start() {
	o.mu.Lock()
	o.active, o.current = true, uuid.Nil()
	o.pids, o.content, o.parsed = map[uint32]bool{}, map[uuid.UUID]int{}, map[uuid.UUID]int{}
	o.queries = nil
	o.mu.Unlock()
	o.single.start()
}
func (o *rebuildStagingObserve) stop() rebuildSingleWriteWork {
	w := o.single.stop()
	o.mu.Lock()
	o.active = false
	o.mu.Unlock()
	return w
}
func (o *rebuildStagingObserve) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	ctx = o.single.TraceQueryStart(ctx, conn, d)
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.active {
		return ctx
	}
	o.pids[conn.PgConn().PID()] = true
	q := rebuildStagingQuery{name: strings.ToLower(strings.TrimSpace(d.SQL))}
	if strings.HasPrefix(d.SQL, "-- name: ") {
		q.name = strings.Fields(d.SQL)[2]
	}
	if at, ok := ctx.Value(rebuildSingleWriteKey{}).(rebuildSingleWriteStart); ok {
		q.insert = at.insert
	}
	for _, arg := range d.Args {
		if id, ok := arg.(uuid.UUID); ok && slices.Contains(o.ids, id) {
			q.source = id
		}
	}
	index := len(o.queries)
	o.queries = append(o.queries, q)
	return context.WithValue(ctx, rebuildStagingQueryKey{}, index)
}
func (o *rebuildStagingObserve) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryEndData) {
	o.single.TraceQueryEnd(ctx, conn, d)
	if index, ok := ctx.Value(rebuildStagingQueryKey{}).(int); ok {
		o.mu.Lock()
		o.queries[index].err = d.Err
		o.mu.Unlock()
	}
}
func (o *rebuildStagingObserve) checkReads(t *testing.T, ids []uuid.UUID) {
	t.Helper()
	if len(o.content) != len(ids) || len(o.parsed) != len(ids) {
		t.Fatalf("REBUILD_STAGING_PARSE_ONCE: Content=%v Parser=%v ids=%v", o.content, o.parsed, ids)
	}
	for _, id := range ids {
		if o.content[id] != 1 || o.parsed[id] != 1 {
			t.Fatalf("REBUILD_STAGING_PARSE_ONCE: id=%s Content=%d Parser=%d", id, o.content[id], o.parsed[id])
		}
	}
}
func (o *rebuildStagingObserve) checkTransaction(t *testing.T, success bool) {
	t.Helper()
	begin, commit, rollback := 0, 0, 0
	for _, q := range o.queries {
		switch q.name {
		case "begin":
			begin++
		case "commit":
			commit++
		case "rollback":
			rollback++
		}
	}
	wantCommit, wantRollback := 0, 1
	if success {
		wantCommit, wantRollback = 1, 0
	}
	if len(o.pids) != 1 || o.pids[0] || begin != 1 || commit != wantCommit || rollback != wantRollback {
		t.Fatalf("REBUILD_STAGING_TX: pids=%v begin/commit/rollback=%d/%d/%d want1/%d/%d", o.pids, begin, commit, rollback, wantCommit, wantRollback)
	}
	terminal := "rollback"
	if success {
		terminal = "commit"
	}
	if len(o.queries) < 2 || o.queries[0].name != "begin" || o.queries[len(o.queries)-1].name != terminal {
		t.Fatalf("REBUILD_STAGING_TX: queries escaped actual BEGIN/%s interval", terminal)
	}
}

type rebuildStagingContents struct {
	linkapp.Contents
	observed *rebuildStagingObserve
}

func (r rebuildStagingContents) Content(ctx context.Context, id uuid.UUID) (string, int, bool, error) {
	r.observed.mu.Lock()
	r.observed.content[id]++
	r.observed.current = id
	r.observed.mu.Unlock()
	return r.Contents.Content(ctx, id)
}

type rebuildStagingParser struct {
	linkapp.Parser
	observed *rebuildStagingObserve
}

func (p rebuildStagingParser) Facts(ctx context.Context, content string) (domain.Facts, error) {
	p.observed.mu.Lock()
	p.observed.parsed[p.observed.current]++
	p.observed.mu.Unlock()
	return p.Parser.Facts(ctx, content)
}
