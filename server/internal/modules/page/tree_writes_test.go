package page_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The wired module's TreeWrites create an attachment's node in a unit on
// the database: the guard sees its file, after runs in the transaction
// with the node written, the observers follow; after's error rolls the
// node back. Check decides as the unit does.
func TestTheAttachmentsWritesReachTheUnit(t *testing.T) {
	f := newFixture(t)
	g, o := &guard{}, &observer{f: f}
	writes := f.module([]page.WriteGuard{g}, nil, []page.PageObserver{o}).TreeWrites()
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: f.alice, SessionID: uuid.NewV7()})
	sum := bytes.Repeat([]byte{1}, 32)
	a := page.NewAsset{NotebookID: f.eng, ParentID: &f.notes, Name: "photo.png", Action: "asset.upload", Client: "web",
		Meta: page.AssetMeta{MIME: "image/png", Bytes: 3, SHA256: sum}}

	if err := writes.Check(ctx, a); err != nil {
		t.Errorf("Check(photo.png) = %v, want nil", err)
	}
	var seen int
	var inTx bool
	n, err := writes.CreateAsset(ctx, a, func(ctx context.Context, n page.NodeInfo) error {
		inTx = postgres.InTx(ctx)
		return postgres.DB(ctx, f.pool).QueryRow(ctx, "SELECT count(*) FROM nodes WHERE id = $1 AND kind = 'asset'", n.ID).Scan(&seen)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !n.Asset || n.Name != "photo.png" || *n.ParentID != f.notes || n.NotebookID != f.eng || n.CreatedBy != f.alice {
		t.Errorf("node = %+v, want photo.png, an attachment under Notes by alice", n)
	}
	if !inTx || seen != 1 {
		t.Errorf("after ran in a transaction: %v, seeing the node %d times; want once, in it", inTx, seen)
	}
	if len(g.steps) != 1 || !g.inTx || g.steps[0].Asset == nil || !bytes.Equal(g.steps[0].Asset.SHA256, sum) {
		t.Errorf("the guard saw %+v, want the attachment's file", g.steps)
	}
	if len(o.events) != 1 || o.events[0].Changes[0].NodeID != n.ID || o.events[0].Changes[0].Revision != 0 {
		t.Errorf("the observer saw %+v, want the attachment created", o.events)
	}

	failed := errors.New("the row failed")
	a.Name = "other.png"
	if _, err := writes.CreateAsset(ctx, a, func(context.Context, page.NodeInfo) error { return failed }); !errors.Is(err, failed) {
		t.Errorf("CreateAsset with after failing = %v, want its error", err)
	}
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE name = 'other.png'"); got != 0 {
		t.Errorf("%d nodes named other.png, want none: after's error rolls the unit back", got)
	}
	a.Name = "PHOTO.png"
	var e *shared.Error
	if err := writes.Check(ctx, a); !errors.As(err, &e) || e.Code != "page.title_taken" {
		t.Errorf("Check(PHOTO.png) = %v, want page.title_taken", err)
	}
}

// The wired module's TreeWrites run an import's units on the database: a
// job acting for alice creates pages and attachments in a changeset of the
// import kind from the job's client, numbering a name taken; a later unit
// merges into it, moving its time on. The content is checked and parsed
// outside the unit; the read port tells a page's depth.
func TestTheImportsWritesReachTheUnit(t *testing.T) {
	f := newFixture(t)
	clock := &steppingClock{at: testNow()}
	f.clock = clock
	o := &observer{f: f}
	writes := f.module(nil, nil, []page.PageObserver{o}).TreeWrites()
	ctx := shared.WithActor(context.Background(), shared.Actor{UserID: f.alice, JobID: uuid.NewV7()})
	if err := writes.CheckContent("# Hi"); err != nil {
		t.Errorf("CheckContent(# Hi) = %v, want nil", err)
	}
	var e *shared.Error
	if err := writes.CheckContent("a\x00b"); !errors.As(err, &e) || e.Code != shared.CodeValidationFailed {
		t.Errorf("CheckContent(NUL) = %v, want 422", err)
	}
	parsed, err := writes.Parse(ctx, "# Hi")
	if err != nil {
		t.Fatal(err)
	}
	spec := page.ImportSpec{NotebookID: f.eng, Action: "transfer.import", Client: page.Client("api")}
	var top, photo page.NodeInfo
	first, err := writes.Import(ctx, spec, func(ctx context.Context, u page.ImportUnit) error {
		var err error
		if top, err = u.CreatePage(ctx, page.ImportedPage{Name: "NOTES", Content: "# Hi", Parsed: parsed}); err != nil {
			return err
		}
		photo, err = u.CreateAsset(ctx, page.ImportedAsset{ParentID: &top.ID, Name: "photo.png",
			Meta: page.AssetMeta{MIME: "image/png", Bytes: 3, SHA256: bytes.Repeat([]byte{1}, 32)}},
			func(context.Context, page.NodeInfo) error { return nil })
		return err
	})
	parsed.Release()
	if err != nil {
		t.Fatal(err)
	}
	if top.Name != "NOTES 2" || top.Asset || !photo.Asset || *photo.ParentID != top.ID {
		t.Errorf("created %+v and %+v, want NOTES 2 numbered past Notes, the photo under it", top, photo)
	}
	if got := f.count(t, "SELECT count(*) FROM changesets WHERE id = $1 AND kind = 'import' AND client = 'api' AND created_by_id = $2",
		first, f.alice); got != 1 {
		t.Errorf("%d changesets %s of an import from the API by alice, want 1", got, first)
	}
	if got := f.count(t, "SELECT count(*) FROM page_contents WHERE node_id = $1 AND content = '# Hi' AND revision = 1", top.ID); got != 1 {
		t.Errorf("%d contents of NOTES 2 at revision 1, want 1", got)
	}
	second, err := writes.Import(ctx, page.ImportSpec{NotebookID: f.eng, Action: "transfer.import", Client: page.Client("api"), Changeset: first},
		func(ctx context.Context, u page.ImportUnit) error {
			_, err := u.CreatePage(ctx, page.ImportedPage{ParentID: &top.ID, Name: "Child"})
			return err
		})
	if err != nil || second != first {
		t.Fatalf("a later unit = %s, %v; want it merged into %s", second, err, first)
	}
	if got := f.count(t, "SELECT count(*) FROM changesets WHERE notebook_id = $1", f.eng); got != 1 {
		t.Errorf("%d changesets, want the one", got)
	}
	if got := f.count(t, "SELECT count(*) FROM changesets WHERE id = $1 AND updated_at > created_at", first); got != 1 {
		t.Errorf("the changeset's time did not move on with the later unit")
	}
	if len(o.events) != 2 {
		t.Errorf("the observer saw %d units, want 2", len(o.events))
	}
	nodes := page.NewExportNodes(f.pool)
	for _, tt := range []struct {
		id    uuid.UUID
		depth int
		ok    bool
	}{{f.notes, 1, true}, {top.ID, 1, true}, {photo.ID, 0, false}, {uuid.NewV7(), 0, false}} {
		if depth, ok, err := nodes.Depth(context.Background(), f.eng, tt.id); err != nil || depth != tt.depth || ok != tt.ok {
			t.Errorf("Depth(%s) = %d, %t, %v; want %d, %t", tt.id, depth, ok, err, tt.depth, tt.ok)
		}
	}
	child := f.count(t, "SELECT count(*) FROM nodes WHERE parent_id = $1 AND name = 'Child'", top.ID)
	var childID uuid.UUID
	if err := f.pool.QueryRow(context.Background(), "SELECT id FROM nodes WHERE parent_id = $1 AND name = 'Child'", top.ID).Scan(&childID); err != nil || child != 1 {
		t.Fatalf("Child: %v", err)
	}
	if depth, ok, err := nodes.Depth(context.Background(), f.eng, childID); err != nil || depth != 2 || !ok {
		t.Errorf("Depth(Child) = %d, %t, %v; want 2", depth, ok, err)
	}
}

// steppingClock moves on a second each time it is read.
type steppingClock struct {
	at time.Time
}

func (c *steppingClock) Now() time.Time {
	c.at = c.at.Add(time.Second)
	return c.at
}

// The template's unit (M10/P1 design 3.6): two pages written by alice in
// the transaction a creation opened, in one changeset of its message.

// seedCtx is alice's request ctx.
func (f fixture) seedCtx() context.Context {
	return shared.WithActor(context.Background(), shared.Actor{UserID: f.alice, SessionID: uuid.NewV7()})
}

// seedOf is a template's unit of eng at at: raw, then AGENTS, each with
// its id, their facts parsed by writes.
func (f fixture) seedOf(t *testing.T, writes page.TreeWrites, at time.Time, raw, agents uuid.UUID) page.SeedSpec {
	t.Helper()
	message := "LLM Wiki template"
	spec := page.SeedSpec{NotebookID: f.eng, At: at, Client: page.Client("web"), Message: &message}
	for _, p := range []page.PageDraft{{ID: raw, Title: "raw", Content: "# Sources\n"}, {ID: agents, Title: "AGENTS", Content: "# Agents\n"}} {
		facts, err := writes.ParseTemplate(p.Content)
		if err != nil {
			t.Fatal(err)
		}
		p.Facts = facts
		spec.Pages = append(spec.Pages, p)
	}
	return spec
}

// seeded counts what the unit wrote of the nodes ids: the nodes, their
// contents and versions, the changesets' items, the changesets of eng and
// the search's queue, its exact rows apart.
func (f fixture) seeded(t *testing.T, ids ...uuid.UUID) (nodes, contents, versions, items, changesets, queued, exact int) {
	t.Helper()
	if err := f.pool.QueryRow(context.Background(), `SELECT
		(SELECT count(*) FROM nodes WHERE id = ANY($1)),
		(SELECT count(*) FROM page_contents WHERE node_id = ANY($1)),
		(SELECT count(*) FROM page_revisions WHERE node_id = ANY($1)),
		(SELECT count(*) FROM changeset_items WHERE node_id = ANY($1)),
		(SELECT count(*) FROM changesets WHERE notebook_id = $2),
		(SELECT count(*) FROM page_search_queue WHERE node_id = ANY($1)),
		(SELECT count(*) FROM page_search_queue WHERE node_id = ANY($1) AND NOT bulk)`, ids, f.eng).
		Scan(&nodes, &contents, &versions, &items, &changesets, &queued, &exact); err != nil {
		t.Fatal(err)
	}
	return nodes, contents, versions, items, changesets, queued, exact
}

// Seed joins the transaction its caller opened: another connection sees
// nothing of it before the commit; the caller's rollback takes it all back,
// the search's queue too; the caller's commit keeps both pages in one
// changeset of the message given, queued exact for the search, and Seed
// answered their ids in order.
func TestASeedJoinsItsCallersTransaction(t *testing.T) {
	f := newFixture(t)
	writes := f.module(nil, nil, nil).TreeWrites()
	tx := postgres.NewTxManager(f.pool, 5*time.Second)
	rollback := errors.New("the caller rolls back")
	for _, commit := range []bool{false, true} {
		raw, agents := uuid.NewV7(), uuid.NewV7()
		spec := f.seedOf(t, writes, testNow(), raw, agents)
		var before int
		var ids []uuid.UUID
		err := tx.WithinTx(f.seedCtx(), func(ctx context.Context) error {
			var err error
			if ids, err = writes.Seed(ctx, spec); err != nil {
				return err
			}
			before, _, _, _, _, _, _ = f.seeded(t, raw, agents)
			if !commit {
				return rollback
			}
			return nil
		})
		nodes, contents, versions, items, changesets, queued, exact := f.seeded(t, raw, agents)
		switch {
		case before != 0:
			t.Errorf("another connection saw %d of the nodes before the commit, want none", before)
		case !commit && (!errors.Is(err, rollback) || nodes+contents+versions+items+changesets+queued != 0):
			t.Errorf("rolled back: %v; %d nodes, %d contents, %d versions, %d items, %d changesets, %d queued; want none",
				err, nodes, contents, versions, items, changesets, queued)
		case commit && (err != nil || nodes != 2 || contents != 2 || versions != 2 || items != 2 || changesets != 1 || exact != 2 ||
			!slices.Equal(ids, []uuid.UUID{raw, agents})):
			t.Errorf("committed: %v; %d nodes, %d contents, %d versions, %d items, %d changesets, %d queued exact, ids %v; "+
				"want 2, 2, 2, 2, 1, 2, raw's and AGENTS's", err, nodes, contents, versions, items, changesets, exact, ids)
		}
	}
	if got := f.count(t, "SELECT count(*) FROM changesets WHERE notebook_id = $1 AND message = 'LLM Wiki template' AND kind = 'edit' "+
		"AND client = 'web' AND created_by_id = $2", f.eng, f.alice); got != 1 {
		t.Errorf("%d changesets of the template's message, an edit from the web by alice; want 1", got)
	}
}

// Seed reads no clock: the changeset, the nodes, their contents, versions
// and items are at the time given, a time the module's clock never says,
// and the clock was not read.
func TestASeedReadsNoClock(t *testing.T) {
	f := newFixture(t)
	clock := &steppingClock{at: testNow()}
	f.clock = clock
	writes := f.module(nil, nil, nil).TreeWrites()
	at := testNow().Add(-time.Minute)
	raw, agents := uuid.NewV7(), uuid.NewV7()
	if err := postgres.NewTxManager(f.pool, 5*time.Second).WithinTx(f.seedCtx(), func(ctx context.Context) error {
		_, err := writes.Seed(ctx, f.seedOf(t, writes, at, raw, agents))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got := f.count(t, "SELECT count(*) FROM changesets WHERE notebook_id = $1 AND created_at = $2 AND updated_at = $2", f.eng, at); got != 1 {
		t.Errorf("%d changesets of eng at %v, want 1", got, at)
	}
	for _, q := range []string{
		"SELECT count(*) FROM nodes WHERE id = ANY($1) AND created_at = $2 AND updated_at = $2",
		"SELECT count(*) FROM page_contents WHERE node_id = ANY($1) AND updated_at = $2",
		"SELECT count(*) FROM page_revisions WHERE node_id = ANY($1) AND created_at = $2 AND updated_at = $2",
		"SELECT count(*) FROM changeset_items WHERE node_id = ANY($1) AND created_at = $2 AND updated_at = $2",
	} {
		if got := f.count(t, q, []uuid.UUID{raw, agents}, at); got != 2 {
			t.Errorf("%s: %d rows at %v, want 2", q, got, at)
		}
	}
	if !clock.at.Equal(testNow()) {
		t.Errorf("the clock moved on to %v: the unit read it", clock.at)
	}
}

// Seed refuses a ctx without a transaction, and no time, writing nothing.
func TestASeedRefusesNoTransactionAndNoTime(t *testing.T) {
	f := newFixture(t)
	writes := f.module(nil, nil, nil).TreeWrites()
	raw, agents := uuid.NewV7(), uuid.NewV7()
	if _, err := writes.Seed(f.seedCtx(), f.seedOf(t, writes, testNow(), raw, agents)); err == nil {
		t.Error("Seed outside a transaction = nil, want an error")
	}
	if err := postgres.NewTxManager(f.pool, 5*time.Second).WithinTx(f.seedCtx(), func(ctx context.Context) error {
		_, err := writes.Seed(ctx, f.seedOf(t, writes, time.Time{}, raw, agents))
		return err
	}); err == nil {
		t.Error("Seed at no time = nil, want an error")
	}
	if nodes, contents, versions, items, changesets, queued, _ := f.seeded(t, raw, agents); nodes+contents+versions+items+changesets+queued != 0 {
		t.Errorf("%d nodes, %d contents, %d versions, %d items, %d changesets, %d queued; want none", nodes, contents, versions, items,
			changesets, queued)
	}
}

// refuseSecond records the steps it sees, and refuses the second.
type refuseSecond struct {
	steps []page.Step
	err   error
}

func (g *refuseSecond) GuardWrite(_ context.Context, s page.Step) error {
	g.steps = append(g.steps, s)
	if len(g.steps) == 2 {
		return g.err
	}
	return nil
}

// Seed's unit runs the guards: both pages' creations reach them, of the
// edit kind and the client given; a guard's refusal of the second rolls
// the caller's transaction back, the first page with it.
func TestASeedRunsTheGuards(t *testing.T) {
	refusal := shared.NewError(shared.KindConflict, "page.refused", "The guard refused it.")
	for _, refuse := range []bool{false, true} {
		f := newFixture(t)
		g := &refuseSecond{}
		if refuse {
			g.err = refusal
		}
		writes := f.module([]page.WriteGuard{g}, nil, nil).TreeWrites()
		raw, agents := uuid.NewV7(), uuid.NewV7()
		err := postgres.NewTxManager(f.pool, 5*time.Second).WithinTx(f.seedCtx(), func(ctx context.Context) error {
			_, err := writes.Seed(ctx, f.seedOf(t, writes, testNow(), raw, agents))
			return err
		})
		if len(g.steps) != 2 {
			t.Fatalf("refused %v: the guard saw %d steps, want both creations", refuse, len(g.steps))
		}
		for i, s := range g.steps {
			if s.Operation != domain.OpCreate || s.Kind != domain.ChangesetEdit || s.Client != "web" ||
				s.Changes[0].NodeID != []uuid.UUID{raw, agents}[i] {
				t.Errorf("refused %v: step %d = %+v, want the creation of the page, an edit from the web", refuse, i, s)
			}
		}
		nodes, _, _, _, changesets, _, _ := f.seeded(t, raw, agents)
		switch {
		case !refuse && (err != nil || nodes != 2 || changesets != 1):
			t.Errorf("Seed() = %v; %d nodes, %d changesets; want both pages in one", err, nodes, changesets)
		case refuse && (!errors.Is(err, refusal) || nodes != 0 || changesets != 0):
			t.Errorf("Seed() refused = %v; %d nodes, %d changesets; want the refusal, nothing", err, nodes, changesets)
		}
	}
}

// Seed passes on the facts it is given, parsed once at start-up, and parses
// nothing itself: a page whose facts are another content's reaches the
// observers with those facts, not its own content's.
func TestASeedPassesTheFactsItIsGiven(t *testing.T) {
	f := newFixture(t)
	o := &observer{f: f}
	writes := f.module(nil, nil, []page.PageObserver{o}).TreeWrites()
	given, err := writes.ParseTemplate("---\nkind: other\n---\n# Other\n")
	if err != nil {
		t.Fatal(err)
	}
	own, err := writes.ParseTemplate("# Sources\n")
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(given, own) {
		t.Fatalf("the two contents' facts are alike, %#v: the test would tell nothing", own)
	}
	if err := postgres.NewTxManager(f.pool, 5*time.Second).WithinTx(f.seedCtx(), func(ctx context.Context) error {
		_, err := writes.Seed(ctx, page.SeedSpec{NotebookID: f.eng, At: testNow(), Client: page.Client("web"),
			Pages: []page.PageDraft{{Title: "raw", Content: "# Sources\n", Facts: given}}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(o.events) != 1 || len(o.events[0].Changes) != 1 {
		t.Fatalf("the observer saw %+v, want one unit of one change", o.events)
	}
	if got := o.events[0].Changes[0].Facts; !reflect.DeepEqual(got, given) {
		t.Errorf("the observer got the facts %#v, want those given, %#v", got, given)
	}
}

// A template's page parsed once at start-up has the facts its content
// would have had parsed by a write: the observers get the same of a Seed
// and of createPage, what the registered extension took from the content
// (its size), as TestTheContentsWritesReachTheUnit has it. The facts are
// read, never written: two Seeds of the same facts, in two notebooks,
// pass the same. ParseTemplate refuses what a write's content check does.
func TestATemplatesParseIsItsContents(t *testing.T) {
	f := newFixture(t)
	md, err := markdown.New([]markdown.Extension{pageMark()})
	if err != nil {
		t.Fatal(err)
	}
	f.md = md
	o := &observer{f: f}
	m := f.module(nil, nil, []page.PageObserver{o})
	writes := m.TreeWrites()
	const content = "# Sources\n\nSee [[Notes]], a #tag and [a link](https://example.com).\n\n- [ ] a task\n"
	facts, err := writes.ParseTemplate(content)
	if err != nil {
		t.Fatal(err)
	}
	other := uuid.NewV7()
	f.exec(t, "INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Ops', $3, $3, $4, $4)",
		other, f.acme, f.alice, testNow())
	tx := postgres.NewTxManager(f.pool, 5*time.Second)
	for _, nb := range []uuid.UUID{f.eng, other} {
		var ids []uuid.UUID
		if err := tx.WithinTx(f.seedCtx(), func(ctx context.Context) error {
			var err error
			ids, err = writes.Seed(ctx, page.SeedSpec{NotebookID: nb, At: testNow(), Client: page.Client("web"),
				Pages: []page.PageDraft{{Title: "raw", Content: content, Facts: facts}}})
			return err
		}); err != nil {
			t.Fatal(err)
		}
		// A page of no id given is written with a new one, which Seed answers.
		if len(ids) != 1 || f.count(t, "SELECT count(*) FROM nodes WHERE id = $1 AND notebook_id = $2 AND name = 'raw'", ids[0], nb) != 1 {
			t.Errorf("Seed in %s answered %v, want the id of the raw it wrote", nb, ids)
		}
	}
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	m.Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: tokenAuth{}}))
	if rec := f.request(router, "session", http.MethodPost, f.createPath(), `{"parent_id":null,"title":"Written","content":"`+
		strings.ReplaceAll(content, "\n", `\n`)+`"}`); rec.Code != http.StatusCreated {
		t.Fatalf("createPage = %d %s", rec.Code, rec.Body)
	}
	if len(o.events) != 3 {
		t.Fatalf("the observer saw %d units, want 3", len(o.events))
	}
	seeds, written := o.events[0].Changes[0].Facts, o.events[2].Changes[0].Facts
	if !reflect.DeepEqual(seeds, written) || !reflect.DeepEqual(o.events[1].Changes[0].Facts, seeds) {
		t.Errorf("the facts the observer got: Seed's %#v, the second Seed's %#v, createPage's %#v; want them alike", seeds,
			o.events[1].Changes[0].Facts, written)
	}
	if facts, _ := seeds.(markdown.Facts); facts.Extracted("page-mark") != len(content) {
		t.Errorf("the extension took %v of the template's page, want its content's size, %d", facts.Extracted("page-mark"), len(content))
	}
	for name, c := range map[string]string{"past 5 MiB": strings.Repeat("a", 5<<20+1), "NUL": "a\x00b"} {
		var e *shared.Error
		if _, err := writes.ParseTemplate(c); !errors.As(err, &e) || e.Code != shared.CodeValidationFailed {
			t.Errorf("ParseTemplate(%s) = %v, want 422", name, err)
		}
	}
}
