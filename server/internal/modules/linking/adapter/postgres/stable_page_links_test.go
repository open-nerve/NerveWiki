package postgresadapter_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type stableAdapterSQLCall struct {
	Name string
	PID  uint32
	Rows int64
	Err  error
	Done bool
}
type stableAdapterTrace struct {
	mu    sync.Mutex
	calls []stableAdapterSQLCall
}
type stableAdapterMarkKey struct{}
type stableAdapterCallKey struct{}
type stableAdapterCallAt struct {
	trace *stableAdapterTrace
	index int
}

func (tr *stableAdapterTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if ctx.Value(stableAdapterMarkKey{}) != tr || !strings.HasPrefix(d.SQL, "-- name: ") {
		return ctx
	}
	fields := strings.Fields(d.SQL)
	if len(fields) < 3 {
		return ctx
	}
	tr.mu.Lock()
	index := len(tr.calls)
	tr.calls = append(tr.calls, stableAdapterSQLCall{Name: fields[2], PID: conn.PgConn().PID()})
	tr.mu.Unlock()
	return context.WithValue(ctx, stableAdapterCallKey{}, stableAdapterCallAt{trace: tr, index: index})
}
func (tr *stableAdapterTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	at, ok := ctx.Value(stableAdapterCallKey{}).(stableAdapterCallAt)
	if !ok || at.trace != tr {
		return
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	c := &tr.calls[at.index]
	c.Rows, c.Err, c.Done = d.CommandTag.RowsAffected(), d.Err, true
}
func (tr *stableAdapterTrace) reset() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.calls = nil
}
func (tr *stableAdapterTrace) snapshot() []stableAdapterSQLCall {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.calls)
}

type stableAdapterFixture struct {
	fixture
	trace *stableAdapterTrace
}

func newStableAdapterFixture(t *testing.T) stableAdapterFixture {
	t.Helper()
	f := newFixture(t)
	tr := &stableAdapterTrace{}
	cfg := f.pool.Config() // Preserve the real pool's connection initialization and query modes.
	cfg.ConnConfig.Tracer = tr
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f.s, f.pool, f.tx = postgresadapter.New(pool), pool, postgres.NewTxManager(pool, time.Second)
	return stableAdapterFixture{fixture: f, trace: tr}
}

func stableAdapterLegacySQL() []string {
	return []string{
		"LockNotebook", "DeleteLinksOf", "DeleteAliasesOf", "DeleteTagsOf", "DeletePropertiesOf", "DeleteIndexedPages",
		"InsertIndexedPage", "InsertLinks", "InsertTags", "InsertProperties", "InsertAliases",
	}
}
func stableAdapterFastSQL() []string {
	return []string{
		"LockNotebook", "InspectPageLinksForRetention", "DeleteMissingPageLinks", "DeleteAliasesOf", "DeleteTagsOf",
		"DeletePropertiesOf", "DeleteIndexedPages", "InsertIndexedPage", "WritePageLinksRetainingEqualPrefiltered",
		"InsertTags", "InsertProperties", "InsertAliases",
	}
}

func (f stableAdapterFixture) run(t *testing.T, notebook uuid.UUID, fn func(context.Context) error) (uint32, error) {
	t.Helper()
	f.trace.reset()
	ctx := context.WithValue(t.Context(), stableAdapterMarkKey{}, f.trace)
	var pid uint32
	err := f.tx.WithinTx(ctx, func(ctx context.Context) error {
		tx, ok := postgres.TxFrom(ctx)
		if !ok {
			return errors.New("stable adapter fixture lost the original transaction")
		}
		pid = tx.Conn().PgConn().PID()
		if err := f.s.Lock(ctx, notebook); err != nil {
			return err
		}
		return fn(ctx)
	})
	return pid, err
}

// These direct Store probes inspect an intermediate baseline, not a freshly
// resolved index. They always roll back instead of committing that baseline.
func (f stableAdapterFixture) rollback(t *testing.T, notebook uuid.UUID, fn func(context.Context) error) uint32 {
	t.Helper()
	before := f.snapshot(t, t.Context(), nil)
	cause := errors.New("end the adapter baseline probe without committing")
	pid, err := f.run(t, notebook, func(ctx context.Context) error {
		if err := fn(ctx); err != nil {
			return err
		}
		return cause
	})
	if !errors.Is(err, cause) {
		t.Fatalf("adapter probe returned %v, want its rollback cause", err)
	}
	if after := f.snapshot(t, t.Context(), nil); after != before {
		t.Errorf("adapter probe rollback changed committed tables: before=%s after=%s", before, after)
	}
	return pid
}

func (f stableAdapterFixture) assertSQL(t *testing.T, want []string, pid uint32, failed string, returned error) {
	t.Helper()
	calls := f.trace.snapshot()
	t.Logf("STABLE_SQL_ACTUAL: %+v", calls)
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Name
		if !c.Done || c.PID == 0 || pid != 0 && c.PID != pid {
			t.Errorf("STABLE_SQL_CONNECTION: call %d=%+v, want completed on PID %d", i, c, pid)
		}
		if c.Name == failed {
			if c.Err == nil || !errors.Is(returned, c.Err) {
				t.Errorf("STABLE_SQL_CAUSE: returned %v, actual %s cause %v", returned, c.Name, c.Err)
			}
		} else if c.Err != nil {
			t.Errorf("STABLE_SQL_UNEXPECTED_ERROR: call %d=%+v", i, c)
		}
	}
	if !slices.Equal(names, want) {
		t.Errorf("STABLE_SQL_ORDER: got %v, want %v", names, want)
	}
}

func stableAdapterValue[T any](v T) *T { return &v }

// Only Dropped's event summary may be a witness. The expected set remains
// complete, as do every persisted-fact and actual native-row assertion.
func stableAssertDroppedWitness(t *testing.T, got, complete []uuid.UUID) {
	t.Helper()
	if len(complete) <= app.MaxEventPages {
		if !slices.Equal(got, complete) || len(complete) == 0 && got != nil {
			t.Fatalf("DROPPED_COMPLETE_SMALL: got%v want%v (nil for empty)", got, complete)
		}
		return
	}
	allowed := make(map[uuid.UUID]bool, len(complete))
	for _, id := range complete {
		allowed[id] = true
	}
	seen := make(map[uuid.UUID]bool, len(got))
	for _, id := range got {
		if id == uuid.Nil() || !allowed[id] || seen[id] {
			t.Fatalf("DROPPED_OVERFLOW_WITNESS: invalid/duplicate target%s in%v", id, got)
		}
		seen[id] = true
	}
	if len(seen) <= app.MaxEventPages {
		t.Fatalf("DROPPED_OVERFLOW_WITNESS: only%d distinct targets for complete%d", len(seen), len(complete))
	}
}

type stableAdapterLink struct {
	Source                    uuid.UUID
	Start, End                int32
	Notebook                  uuid.UUID
	Kind                      string
	Property                  *string
	Target                    string
	Anchor, Display, Key, Alt *string
	Resolved                  *uuid.UUID
	Ambiguous, Aliases, Asset bool
}

func (f stableAdapterFixture) links(t *testing.T, ctx context.Context, source uuid.UUID) []stableAdapterLink {
	t.Helper()
	rows, err := postgres.DB(ctx, f.pool).Query(ctx, `SELECT source_id, range_start, range_end, notebook_id, kind,
 property_key, target, anchor, display, target_key, target_alt_key, resolved_id, ambiguous, aliases, resolved_asset
 FROM page_links WHERE source_id=$1 ORDER BY range_start`, source)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []stableAdapterLink
	for rows.Next() {
		var r stableAdapterLink
		if err := rows.Scan(&r.Source, &r.Start, &r.End, &r.Notebook, &r.Kind, &r.Property, &r.Target,
			&r.Anchor, &r.Display, &r.Key, &r.Alt, &r.Resolved, &r.Ambiguous, &r.Aliases, &r.Asset); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (f stableAdapterFixture) textRows(t *testing.T, ctx context.Context, sql string, args ...any) [][]string {
	t.Helper()
	rows, err := postgres.DB(ctx, f.pool).Query(ctx, sql, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			t.Fatal(err)
		}
		var row []string
		for _, v := range values {
			row = append(row, text(v))
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (f stableAdapterFixture) snapshot(t *testing.T, ctx context.Context, source *uuid.UUID) string {
	t.Helper()
	var out string
	err := postgres.DB(ctx, f.pool).QueryRow(ctx, `SELECT jsonb_build_object(
 'heads', (SELECT coalesce(jsonb_agg(to_jsonb(h) ORDER BY node_id),'[]'::jsonb) FROM indexed_pages h WHERE $1::uuid IS NULL OR node_id=$1),
 'links', (SELECT coalesce(jsonb_agg(to_jsonb(l) ORDER BY source_id,range_start),'[]'::jsonb) FROM page_links l WHERE $1::uuid IS NULL OR source_id=$1),
 'tags', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY source_id,tag_key),'[]'::jsonb) FROM page_tags t WHERE $1::uuid IS NULL OR source_id=$1),
 'properties', (SELECT coalesce(jsonb_agg(to_jsonb(p) ORDER BY source_id,position),'[]'::jsonb) FROM page_properties p WHERE $1::uuid IS NULL OR source_id=$1),
 'aliases', (SELECT coalesce(jsonb_agg(to_jsonb(a) ORDER BY source_id,alias_key),'[]'::jsonb) FROM page_aliases a WHERE $1::uuid IS NULL OR source_id=$1)
 )::text`, source).Scan(&out)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func (f stableAdapterFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if tag, err := f.pool.Exec(t.Context(), sql, args...); err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("fixture edit: rows=%d error=%v", tag.RowsAffected(), err)
	}
}

func (f stableAdapterFixture) seed(t *testing.T, p app.Page, facts domain.Facts, resolutions []app.Link) {
	t.Helper()
	if err := f.s.AddPage(t.Context(), p, facts); err != nil {
		t.Fatal(err)
	}
	if len(resolutions) > 0 {
		for _, l := range resolutions {
			if l.Resolution.ID == (uuid.UUID{}) {
				t.Fatal("stable adapter fixture requires a nonempty old baseline")
			}
		}
		if err := f.s.SetResolutions(t.Context(), resolutions); err != nil {
			t.Fatal(err)
		}
	}
}

// Independent literal inputs and persisted facts: neither calls the parser or
// linksParams to construct its expected representation.
func stableAdapterInput(at int) domain.Link {
	return domain.Link{Kind: "wikilink", Property: "refs.0", Target: "Note.md", Anchor: "Heading", Display: "Caption", Start: at, End: at + 7}
}
func stableAdapterFact(p app.Page, at int) stableAdapterLink {
	return stableAdapterLink{Source: p.ID, Start: int32(at), End: int32(at + 7), Notebook: p.NotebookID,
		Kind: "wikilink", Property: stableAdapterValue("refs.0"), Target: "Note.md", Anchor: stableAdapterValue("Heading"),
		Display: stableAdapterValue("Caption"), Key: stableAdapterValue("note"), Alt: stableAdapterValue("note.md")}
}

func (f stableAdapterFixture) assertLinks(t *testing.T, ctx context.Context, source uuid.UUID, want []stableAdapterLink) {
	t.Helper()
	if got := f.links(t, ctx, source); !reflect.DeepEqual(got, want) {
		t.Errorf("STABLE_FACT_BASELINE: got %+v, want %+v", got, want)
	}
}

func TestStablePageLinksCompareEveryPersistedFact(t *testing.T) {
	f := newStableAdapterFixture(t)
	p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 3}
	type change struct {
		name   string
		input  func(*domain.Link)
		want   func(*stableAdapterLink)
		oldSQL string
		keep   bool
	}
	cases := []change{
		{name: "identical", keep: true},
		{name: "nonpersistent_quote_and_table", input: func(l *domain.Link) { l.Quote, l.InTable = '\'', true }, keep: true},
		{name: "range_start", input: func(l *domain.Link) { l.Start++ }, want: func(r *stableAdapterLink) { r.Start++ }},
		{name: "range_end", input: func(l *domain.Link) { l.End++ }, want: func(r *stableAdapterLink) { r.End++ }},
		{name: "kind_embed", input: func(l *domain.Link) { l.Kind = "embed" }, want: func(r *stableAdapterLink) { r.Kind = "embed" }},
		{name: "kind_link", input: func(l *domain.Link) { l.Kind = "link" }, want: func(r *stableAdapterLink) { r.Kind = "link" }},
		{name: "kind_image", input: func(l *domain.Link) { l.Kind = "image" }, want: func(r *stableAdapterLink) { r.Kind = "image" }},
		{name: "raw_case", input: func(l *domain.Link) { l.Target = "NOTE.md" }, want: func(r *stableAdapterLink) { r.Target = "NOTE.md" }},
		{name: "raw_unicode", oldSQL: "target='Straße.md', target_key='strasse', target_alt_key='strasse.md'",
			input: func(l *domain.Link) { l.Target = "STRASSE.md" }, want: func(r *stableAdapterLink) {
				r.Target, r.Key, r.Alt = "STRASSE.md", stableAdapterValue("strasse"), stableAdapterValue("strasse.md")
			}},
		{name: "property", input: func(l *domain.Link) { l.Property = "refs.1" }, want: func(r *stableAdapterLink) { r.Property = stableAdapterValue("refs.1") }},
		{name: "property_to_body", input: func(l *domain.Link) { l.Property = "" }, want: func(r *stableAdapterLink) { r.Property = nil }},
		{name: "property_to_empty", input: func(l *domain.Link) { l.Property, l.Quote = "", '\'' }, want: func(r *stableAdapterLink) { r.Property = stableAdapterValue("") }},
		{name: "property_null_to_empty", oldSQL: "property_key=NULL", input: func(l *domain.Link) { l.Property, l.Quote = "", '\'' }, want: func(r *stableAdapterLink) { r.Property = stableAdapterValue("") }},
		{name: "property_empty_to_null", oldSQL: "property_key=''", input: func(l *domain.Link) { l.Property = "" }, want: func(r *stableAdapterLink) { r.Property = nil }},
		{name: "anchor", input: func(l *domain.Link) { l.Anchor = "Other" }, want: func(r *stableAdapterLink) { r.Anchor = stableAdapterValue("Other") }},
		{name: "anchor_empty_to_null", oldSQL: "anchor=''", input: func(l *domain.Link) { l.Anchor = "" }, want: func(r *stableAdapterLink) { r.Anchor = nil }},
		{name: "display", input: func(l *domain.Link) { l.Display = "Other" }, want: func(r *stableAdapterLink) { r.Display = stableAdapterValue("Other") }},
		{name: "display_empty_to_null", oldSQL: "display=''", input: func(l *domain.Link) { l.Display = "" }, want: func(r *stableAdapterLink) { r.Display = nil }},
		{name: "target_key", oldSQL: "target_key='wrong'"},
		{name: "target_alt_key", oldSQL: "target_alt_key='wrong.md'"},
		{name: "target_key_empty_to_null", oldSQL: "target='A//B', target_key='', target_alt_key=NULL",
			input: func(l *domain.Link) { l.Target = "A//B" }, want: func(r *stableAdapterLink) { r.Target, r.Key, r.Alt = "A//B", nil, nil }},
		{name: "target_alt_key_empty_to_null", oldSQL: "target='Note', target_key='note', target_alt_key=''",
			input: func(l *domain.Link) { l.Target = "Note" }, want: func(r *stableAdapterLink) { r.Target, r.Alt = "Note", nil }},
		{name: "aliases", input: func(l *domain.Link) { l.Aliases = true }, want: func(r *stableAdapterLink) { r.Aliases = true }},
		{name: "unparseable_is_not_invalid_input", oldSQL: "target='A//B', target_key=NULL, target_alt_key=NULL", keep: true,
			input: func(l *domain.Link) { l.Target = "A//B" }, want: func(r *stableAdapterLink) { r.Target, r.Key, r.Alt = "A//B", nil, nil }},
	}
	old, incoming := domain.Facts{FrontmatterValid: true}, domain.Facts{FrontmatterValid: true}
	var seeded, want []stableAdapterLink
	var resolutions []app.Link
	var targets []uuid.UUID
	for i, tc := range cases {
		at := 100*i + 20
		old.Links = append(old.Links, stableAdapterInput(at))
		l, r := stableAdapterInput(at), stableAdapterFact(p, at)
		id := uuid.NewV7()
		targets = append(targets, id)
		base := r
		base.Resolved, base.Ambiguous, base.Asset = stableAdapterValue(id), true, true
		seeded = append(seeded, base)
		resolutions = append(resolutions, app.Link{SourceID: p.ID, Start: at, Resolution: domain.Resolution{ID: id, Ambiguous: true, Asset: true}})
		if tc.input != nil {
			tc.input(&l)
		}
		if tc.want != nil {
			tc.want(&r)
		}
		if tc.keep {
			r.Resolved, r.Ambiguous, r.Asset = stableAdapterValue(id), true, true
		}
		incoming.Links, want = append(incoming.Links, l), append(want, r)
	}
	f.seed(t, p, old, resolutions)
	if got := f.links(t, t.Context(), p.ID); !reflect.DeepEqual(got, seeded) {
		t.Fatalf("STABLE_FIXTURE_SEED: got %+v, want %+v", got, seeded)
	}
	for i, tc := range cases {
		if tc.oldSQL != "" {
			f.exec(t, "UPDATE page_links SET "+tc.oldSQL+" WHERE source_id=$1 AND range_start=$2", p.ID, 100*i+20)
		}
	}
	// A deliberately non-adjacent revision proves no Revision-1 health claim.
	p.Revision = 9
	pid := f.rollback(t, p.NotebookID, func(ctx context.Context) error {
		dropped, err := f.s.ReplacePage(ctx, p, incoming)
		if err != nil {
			return err
		}
		got := f.links(t, ctx, p.ID)
		if len(got) != len(want) {
			t.Fatalf("fact matrix has %d rows, want %d", len(got), len(want))
		}
		for i, r := range want {
			if !reflect.DeepEqual(got[i], r) {
				t.Errorf("STABLE_FACT_BASELINE %s: got %+v, want %+v", cases[i].name, got[i], r)
			}
		}
		slices.SortFunc(targets, uuid.UUID.Compare)
		stableAssertDroppedWitness(t, dropped.Targets, targets)
		if dropped.AliasKeys != nil {
			t.Errorf("old alias keys=%v, want nil", dropped.AliasKeys)
		}
		if got := f.textRows(t, ctx, `SELECT revision::text,extractor::text,frontmatter_valid::text FROM indexed_pages WHERE node_id=$1`, p.ID); !reflect.DeepEqual(got, [][]string{{"9", "2", "true"}}) {
			t.Errorf("complete new head=%v", got)
		}
		return nil
	})
	f.assertSQL(t, stableAdapterFastSQL(), pid, "", nil)
}

func TestStablePageLinksReturnAllOldTargetsAndRebuildSideFacts(t *testing.T) {
	f := newStableAdapterFixture(t)
	p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
	q := app.Page{ID: uuid.NewV7(), NotebookID: f.ops, Revision: 1}
	a, b, c := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	if a == (uuid.UUID{}) || b == (uuid.UUID{}) || c == (uuid.UUID{}) || a == b || a == c || b == c {
		t.Fatal("fixture requires three distinct nonempty old targets")
	}
	old := domain.Facts{FrontmatterValid: true,
		Links: []domain.Link{
			{Kind: "wikilink", Target: "Note", Start: 10, End: 14},
			{Kind: "embed", Target: "Photo.png", Start: 30, End: 39},
			{Kind: "link", Target: "Old.md", Start: 50, End: 56},
			{Kind: "image", Target: "A//B", Start: 70, End: 74},
		},
		Tags:       []domain.Tag{{Key: "keep", Name: "Keep", Count: 2}, {Key: "old", Name: "Old", Count: 1}},
		Properties: []domain.Property{{Key: "a", Value: []byte(`[1]`)}, {Key: "b", Value: []byte(`{"old":true}`)}},
		Aliases:    []domain.Alias{{Key: "keep", Name: "Keep"}, {Key: "old", Name: "Old"}},
	}
	f.seed(t, p, old, []app.Link{
		{SourceID: p.ID, Start: 10, Resolution: domain.Resolution{ID: a}},
		{SourceID: p.ID, Start: 30, Resolution: domain.Resolution{ID: a, Ambiguous: true, Asset: true}},
		{SourceID: p.ID, Start: 50, Resolution: domain.Resolution{ID: b}},
		{SourceID: p.ID, Start: 70, Resolution: domain.Resolution{ID: c, Ambiguous: true}},
	})
	f.seed(t, q, old, []app.Link{{SourceID: q.ID, Start: 10, Resolution: domain.Resolution{ID: b}}})
	otherBefore := f.snapshot(t, t.Context(), &q.ID)
	incoming := domain.Facts{FrontmatterValid: true,
		Links: []domain.Link{
			{Kind: "wikilink", Target: "Note", Start: 10, End: 14},
			{Kind: "embed", Target: "Other.png", Start: 30, End: 39},
			{Kind: "image", Target: "A//B", Start: 70, End: 74},
			{Kind: "link", Target: "Tail", Start: 90, End: 94},
		},
		Tags:       []domain.Tag{{Key: "keep", Name: "KEEP", Count: 7}, {Key: "new", Name: "New", Count: 3}},
		Properties: []domain.Property{{Key: "b", Value: []byte(`["new",2]`)}, {Key: "a", Value: []byte(`null`)}},
		Aliases:    []domain.Alias{{Key: "keep", Name: "KEEP"}, {Key: "new", Name: "New"}},
	}
	p.Revision = 2
	pid := f.rollback(t, p.NotebookID, func(ctx context.Context) error {
		dropped, err := f.s.ReplacePage(ctx, p, incoming)
		if err != nil {
			return err
		}
		f.assertLinks(t, ctx, p.ID, []stableAdapterLink{
			{Source: p.ID, Start: 10, End: 14, Notebook: f.eng, Kind: "wikilink", Target: "Note", Key: stableAdapterValue("note"), Resolved: stableAdapterValue(a)},
			{Source: p.ID, Start: 30, End: 39, Notebook: f.eng, Kind: "embed", Target: "Other.png", Key: stableAdapterValue("other.png")},
			{Source: p.ID, Start: 70, End: 74, Notebook: f.eng, Kind: "image", Target: "A//B", Resolved: stableAdapterValue(c), Ambiguous: true},
			{Source: p.ID, Start: 90, End: 94, Notebook: f.eng, Kind: "link", Target: "Tail", Key: stableAdapterValue("tail")},
		})
		targets := []uuid.UUID{a, b, c}
		slices.SortFunc(targets, uuid.UUID.Compare)
		if !reflect.DeepEqual(dropped, app.Dropped{Targets: targets, AliasKeys: []string{"keep", "old"}}) {
			t.Errorf("full old D/aliases=%+v, want all retained/changed/deleted targets and old keys", dropped)
		}
		for _, check := range []struct {
			sql  string
			want [][]string
		}{
			{`SELECT notebook_id::text,revision::text,extractor::text,frontmatter_valid::text FROM indexed_pages WHERE node_id=$1`, [][]string{{f.eng.String(), "2", "2", "true"}}},
			{`SELECT tag_key,notebook_id::text,tag,count::text FROM page_tags WHERE source_id=$1 ORDER BY tag_key`, [][]string{{"keep", f.eng.String(), "KEEP", "7"}, {"new", f.eng.String(), "New", "3"}}},
			{`SELECT position::text,notebook_id::text,key,value::text FROM page_properties WHERE source_id=$1 ORDER BY position`, [][]string{{"0", f.eng.String(), "b", `["new", 2]`}, {"1", f.eng.String(), "a", "null"}}},
			{`SELECT alias_key,notebook_id::text,alias FROM page_aliases WHERE source_id=$1 ORDER BY alias_key`, [][]string{{"keep", f.eng.String(), "KEEP"}, {"new", f.eng.String(), "New"}}},
		} {
			if got := f.textRows(t, ctx, check.sql, p.ID); !reflect.DeepEqual(got, check.want) {
				t.Errorf("side facts %s: got %v, want %v", check.sql, got, check.want)
			}
		}
		if other := f.snapshot(t, ctx, &q.ID); other != otherBefore {
			t.Errorf("another source/notebook changed: %s", other)
		}
		return nil
	})
	f.assertSQL(t, stableAdapterFastSQL(), pid, "", nil)
}

func TestStablePageLinksNormalizeEmptyReplacementDrops(t *testing.T) {
	for _, name := range []string{"nil", "empty", "clear"} {
		t.Run(name, func(t *testing.T) {
			f := newStableAdapterFixture(t)
			p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
			old := domain.Facts{FrontmatterValid: true}
			var resolutions []app.Link
			wantDrop := app.Dropped{}
			if name == "clear" {
				a := uuid.NewV7()
				old.Links = []domain.Link{stableAdapterInput(20)}
				old.Tags = []domain.Tag{{Key: "old", Name: "Old", Count: 1}}
				old.Properties = []domain.Property{{Key: "old", Value: []byte(`true`)}}
				old.Aliases = []domain.Alias{{Key: "old", Name: "Old"}}
				resolutions = []app.Link{{SourceID: p.ID, Start: 20, Resolution: domain.Resolution{ID: a}}}
				wantDrop = app.Dropped{Targets: []uuid.UUID{a}, AliasKeys: []string{"old"}}
			}
			f.seed(t, p, old, resolutions)
			incoming := domain.Facts{}
			if name == "empty" {
				incoming.Links = []domain.Link{}
			}
			p.Revision = 2
			pid := f.rollback(t, p.NotebookID, func(ctx context.Context) error {
				dropped, err := f.s.ReplacePage(ctx, p, incoming)
				if err != nil {
					return err
				}
				if !reflect.DeepEqual(dropped, wantDrop) {
					t.Errorf("nil-normalized drop=%+v, want %#v", dropped, wantDrop)
				}
				f.assertLinks(t, ctx, p.ID, nil)
				for _, table := range []string{"page_tags", "page_properties", "page_aliases"} {
					if got := f.textRows(t, ctx, "SELECT count(*)::text FROM "+table+" WHERE source_id=$1", p.ID); !reflect.DeepEqual(got, [][]string{{"0"}}) {
						t.Errorf("%s not cleared: %v", table, got)
					}
				}
				if got := f.textRows(t, ctx, `SELECT revision::text,extractor::text,frontmatter_valid::text FROM indexed_pages WHERE node_id=$1`, p.ID); !reflect.DeepEqual(got, [][]string{{"2", "2", "false"}}) {
					t.Errorf("empty replacement head=%v", got)
				}
				return nil
			})
			f.assertSQL(t, stableAdapterFastSQL(), pid, "", nil)
		})
	}
}

func TestStablePageLinksRepairIneligibleStoredStateThroughLegacy(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		zero      bool
	}{
		{"missing_head", `DELETE FROM indexed_pages WHERE node_id=$1`, false},
		{"foreign_head", `UPDATE indexed_pages SET notebook_id=$2 WHERE node_id=$1`, false},
		{"old_extractor", `UPDATE indexed_pages SET extractor=1 WHERE node_id=$1`, false},
		{"nonnull_zero_resolution", `UPDATE page_links SET resolved_id='00000000-0000-0000-0000-000000000000'::uuid WHERE source_id=$1`, true},
		{"foreign_links", `UPDATE page_links SET notebook_id=$2 WHERE source_id=$1`, false},
		{"foreign_tags", `UPDATE page_tags SET notebook_id=$2 WHERE source_id=$1`, false},
		{"foreign_properties", `UPDATE page_properties SET notebook_id=$2 WHERE source_id=$1`, false},
		{"foreign_aliases", `UPDATE page_aliases SET notebook_id=$2 WHERE source_id=$1`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStableAdapterFixture(t)
			p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
			a := uuid.NewV7()
			facts := domain.Facts{FrontmatterValid: true, Links: []domain.Link{stableAdapterInput(20)},
				Tags: []domain.Tag{{Key: "tag", Name: "Tag", Count: 2}}, Properties: []domain.Property{{Key: "p", Value: []byte(`{"x":1}`)}}, Aliases: []domain.Alias{{Key: "old", Name: "Old"}}}
			f.seed(t, p, facts, []app.Link{{SourceID: p.ID, Start: 20, Resolution: domain.Resolution{ID: a, Ambiguous: true, Asset: true}}})
			args := []any{p.ID}
			if strings.Contains(tc.sql, "$2") {
				args = append(args, f.ops)
			}
			f.exec(t, tc.sql, args...)
			p.Revision = 2
			pid := f.rollback(t, p.NotebookID, func(ctx context.Context) error {
				dropped, err := f.s.ReplacePage(ctx, p, facts)
				if err != nil {
					return err
				}
				f.assertLinks(t, ctx, p.ID, []stableAdapterLink{stableAdapterFact(p, 20)})
				id := a
				if tc.zero {
					id = uuid.UUID{}
				}
				if !reflect.DeepEqual(dropped, app.Dropped{Targets: []uuid.UUID{id}, AliasKeys: []string{"old"}}) {
					t.Errorf("legacy D/aliases=%+v", dropped)
				}
				for _, table := range []string{"indexed_pages", "page_links", "page_tags", "page_properties", "page_aliases"} {
					key := "source_id"
					if table == "indexed_pages" {
						key = "node_id"
					}
					if got := f.textRows(t, ctx, "SELECT notebook_id::text FROM "+table+" WHERE "+key+"=$1", p.ID); !reflect.DeepEqual(got, [][]string{{f.eng.String()}}) {
						t.Errorf("%s repair=%v", table, got)
					}
				}
				if got := f.textRows(t, ctx, `SELECT revision::text,extractor::text FROM indexed_pages WHERE node_id=$1`, p.ID); !reflect.DeepEqual(got, [][]string{{"2", "2"}}) {
					t.Errorf("repaired head=%v", got)
				}
				return nil
			})
			want := append([]string{"LockNotebook", "InspectPageLinksForRetention"}, stableAdapterLegacySQL()[1:]...)
			f.assertSQL(t, want, pid, "", nil)
		})
	}
}

func TestStablePageLinksKeepAcceptedLegacyInputs(t *testing.T) {
	for _, name := range []string{"zero_source", "zero_notebook", "wrapped_revision", "wrapped_ranges", "wrapped_tag_count"} {
		t.Run(name, func(t *testing.T) {
			f := newStableAdapterFixture(t)
			p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
			if name == "zero_source" {
				p.ID = uuid.UUID{}
			}
			if name == "zero_notebook" {
				p.NotebookID = uuid.UUID{}
			}
			a := uuid.NewV7()
			facts := domain.Facts{FrontmatterValid: true, Links: []domain.Link{stableAdapterInput(20)}, Tags: []domain.Tag{{Key: "tag", Name: "Tag", Count: 2}}}
			f.seed(t, p, facts, []app.Link{{SourceID: p.ID, Start: 20, Resolution: domain.Resolution{ID: a, Asset: true}}})
			p.Revision = 2
			large := int64(1) << 32
			switch name {
			case "wrapped_revision":
				p.Revision = int(large) + 2
			case "wrapped_ranges":
				facts.Links[0].Start, facts.Links[0].End = int(large)+20, int(large)+27
			case "wrapped_tag_count":
				facts.Tags[0].Count = int(large) + 2
			}
			pid := f.rollback(t, p.NotebookID, func(ctx context.Context) error {
				dropped, err := f.s.ReplacePage(ctx, p, facts)
				if err != nil {
					return err
				}
				f.assertLinks(t, ctx, p.ID, []stableAdapterLink{stableAdapterFact(p, 20)})
				if !reflect.DeepEqual(dropped, app.Dropped{Targets: []uuid.UUID{a}}) {
					t.Errorf("legacy accepted input drop=%+v", dropped)
				}
				if got := f.textRows(t, ctx, `SELECT revision::text FROM indexed_pages WHERE node_id=$1`, p.ID); !reflect.DeepEqual(got, [][]string{{"2"}}) {
					t.Errorf("legacy revision conversion=%v", got)
				}
				if got := f.textRows(t, ctx, `SELECT count::text FROM page_tags WHERE source_id=$1`, p.ID); !reflect.DeepEqual(got, [][]string{{"2"}}) {
					t.Errorf("legacy count conversion=%v", got)
				}
				return nil
			})
			f.assertSQL(t, stableAdapterLegacySQL(), pid, "", nil)
		})
	}
}

func TestStablePageLinksStayLegacyOutsideATransaction(t *testing.T) {
	for _, name := range []string{"pool", "connection"} {
		t.Run(name, func(t *testing.T) {
			f := newStableAdapterFixture(t)
			p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
			a := uuid.NewV7()
			facts := domain.Facts{FrontmatterValid: true, Links: []domain.Link{stableAdapterInput(20)}}
			f.seed(t, p, facts, []app.Link{{SourceID: p.ID, Start: 20, Resolution: domain.Resolution{ID: a, Ambiguous: true, Asset: true}}})
			ctx := context.WithValue(t.Context(), stableAdapterMarkKey{}, f.trace)
			var pid uint32
			if name == "connection" {
				conn, err := f.pool.Acquire(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Release()
				ctx = postgres.OnConn(ctx, conn)
				pid = conn.Conn().PgConn().PID()
			}
			if postgres.InTx(ctx) {
				t.Fatal("fixture unexpectedly carries a transaction")
			}
			f.trace.reset()
			p.Revision = 2
			dropped, err := f.s.ReplacePage(ctx, p, facts)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(dropped, app.Dropped{Targets: []uuid.UUID{a}}) {
				t.Errorf("outside-Tx old D=%+v", dropped)
			}
			f.assertLinks(t, t.Context(), p.ID, []stableAdapterLink{stableAdapterFact(p, 20)})
			if got := f.textRows(t, t.Context(), `SELECT revision::text FROM indexed_pages WHERE node_id=$1`, p.ID); !reflect.DeepEqual(got, [][]string{{strconv.Itoa(p.Revision)}}) {
				t.Errorf("outside-Tx head=%v", got)
			}
			f.assertSQL(t, stableAdapterLegacySQL()[1:], pid, "", nil)
		})
	}
}
