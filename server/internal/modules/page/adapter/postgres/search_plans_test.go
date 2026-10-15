package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// corpusHan are common Chinese characters, the most common first; corpusWords
// English words, the common ones first, then rarer ones a search looks for.
const (
	corpusHan = "的一是在不了有和人这中大为上个国我以要他时来用们生到作地于出就分对成会可主发年动同工也能下过子说产种面而方后多定行" +
		"学法所民得经十三之进着等部度家电力里如水化高自二理起小物现实加量都两体制机当使点从业本去把性好应开它合还因由其些然前外天政四日那" +
		"社义事平形相全表间样与关各重新线内数正心反你明看原又么利比或但质气第向道命此变条只没结解问意建月公无系军很情者最立代想已通并提直" +
		"题党程展五果料象员革位入常文总次品式活设及管特件长求老头基资边流路级少图山统接知较将组见计别她手角期根论运农指几九区强放决西被干" +
		"做必战先回则任取据处府研"
	corpusWords = "the of and to in is that for it as with was on be by this are or from at an which have not but they his her she" +
		" he we you all can more one other were their there been has when who will no if out so up said what its about into than" +
		" them only new some could time these two may first then do any like my now over such our man me even most made after" +
		" also did many before must through back years where much your way well down should because each just those people how" +
		" too little state good very make world still own see men work long get here between both life being under never day" +
		" same another know while last might us great old year off come since against go came right used take three" +
		" server network memory cache query index token parser compiler thread process socket buffer stream vector matrix graph" +
		" python golang rust java linux docker cluster shard replica kernel database rapid latency"
)

// searchCorpus writes synthetic pages (M8/P5 design 3.9's guard): each
// named by two or three words or characters and its number, its content
// of about size bytes, runs of the characters between sentences of the
// words, each drawn by a Zipf distribution, from a fixed seed.
type searchCorpus struct {
	r           *rand.Rand
	han, words  []string
	hanZ, wordZ *rand.Zipf
}

func newSearchCorpus(seed uint64) *searchCorpus {
	r := rand.New(rand.NewPCG(seed, 1))
	var han []string
	for _, c := range corpusHan {
		if !slices.Contains(han, string(c)) {
			han = append(han, string(c))
		}
	}
	words := strings.Fields(corpusWords)
	return &searchCorpus{r: r, han: han, words: words, hanZ: rand.NewZipf(r, 1.1, 1, uint64(len(han)-1)),
		wordZ: rand.NewZipf(r, 1.1, 1, uint64(len(words)-1))}
}

// name is the name of the page i.
func (c *searchCorpus) name(i int) string {
	var b strings.Builder
	for k := range 2 + c.r.IntN(2) {
		if i%2 == 0 {
			b.WriteString(c.han[c.hanZ.Uint64()])
			continue
		}
		if k > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(c.words[c.wordZ.Uint64()])
	}
	return fmt.Sprintf("%s %d", b.String(), i)
}

// content is a page's content of about size bytes.
func (c *searchCorpus) content(size int) string {
	var b strings.Builder
	for b.Len() < size {
		for range 10 + c.r.IntN(30) {
			b.WriteString(c.han[c.hanZ.Uint64()])
		}
		b.WriteString("。\n")
		for k := range 5 + c.r.IntN(10) {
			if k > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(c.words[c.wordZ.Uint64()])
		}
		b.WriteString(". ")
	}
	return b.String()
}

// pages adds n pages of the corpus to notebook at its root, through SQL,
// the page i changed i seconds after now(), and answers their ids by id.
func (c *searchCorpus) pages(t *testing.T, f fixture, notebook uuid.UUID, n, size int) []uuid.UUID {
	t.Helper()
	ctx := context.Background()
	names, contents, hashes := make([]string, n), make([]string, n), make([][]byte, n)
	for i := range n {
		names[i], contents[i] = c.name(i), c.content(size)
		sum := sha256.Sum256([]byte(contents[i]))
		hashes[i] = sum[:]
	}
	rows, err := f.pool.Query(ctx, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id,
			updated_by_id, created_at, updated_at)
		SELECT uuidv7(), $1, NULL, 'page', x.name, lower(x.name), x.k, $2, $2, $3::timestamptz, $3::timestamptz + x.k * interval '1 second'
		FROM unnest($4::text[]) WITH ORDINALITY AS x(name, k)
		RETURNING id, sort_order::integer`, notebook, f.alice, now(), names)
	if err != nil {
		t.Fatal(err)
	}
	type created struct {
		id uuid.UUID
		k  int
	}
	made, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (created, error) {
		var c created
		return c, r.Scan(&c.id, &c.k)
	})
	if err != nil || len(made) != n {
		t.Fatalf("%d pages made of %d, %v", len(made), n, err)
	}
	ids := make([]uuid.UUID, n)
	for _, m := range made {
		ids[m.k-1] = m.id
	}
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT x.id, x.content, 1, x.hash, octet_length(x.content), $4, $5::timestamptz + x.k * interval '1 second'
		FROM unnest($1::uuid[], $2::text[], $3::bytea[]) WITH ORDINALITY AS x(id, content, hash, k)`, ids, contents, hashes, f.alice, now())
	return slices.SortedFunc(slices.Values(ids), uuid.UUID.Compare)
}

// attachments adds n attachments to notebook at its root, named by the
// words, through SQL, and writes their rows as the job builds them.
func (c *searchCorpus) attachments(t *testing.T, f fixture, notebook uuid.UUID, n int) {
	t.Helper()
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("%s %d.png", c.words[c.wordZ.Uint64()], i)
	}
	rows, err := f.pool.Query(context.Background(), `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order,
			created_by_id, updated_by_id, created_at, updated_at)
		SELECT uuidv7(), $1, NULL, 'asset', x.name, lower(x.name), x.k, $2, $2, $3, $3 FROM unnest($4::text[]) WITH ORDINALITY AS x(name, k)
		RETURNING id, name`, notebook, f.alice, now(), names)
	if err != nil {
		t.Fatal(err)
	}
	built, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.SearchRow, error) {
		row := app.SearchRow{Tokenizer: domain.SearchTokenizer, BuiltAt: now()}
		if err := r.Scan(&row.NodeID, &row.Name); err != nil {
			return row, err
		}
		row.Title, _ = domain.SearchTokens(row.Name, nil)
		return row, nil
	})
	if err != nil || len(built) != n {
		t.Fatalf("%d attachments made of %d, %v", len(built), n, err)
	}
	f.writeRows(t, built)
}

// planNode is a node of a plan, as EXPLAIN (FORMAT JSON, VERBOSE) gives it.
type planNode struct {
	Type       string     `json:"Node Type"`
	Relation   string     `json:"Relation Name"`
	Alias      string     `json:"Alias"`
	Index      string     `json:"Index Name"`
	IndexCond  string     `json:"Index Cond"`
	Function   string     `json:"Function Name"`
	Filter     string     `json:"Filter"`
	JoinFilter string     `json:"Join Filter"`
	Output     []string   `json:"Output"`
	Plans      []planNode `json:"Plans"`
}

// planTreeOf is the plan of s, scans of whole tables off, and its JSON.
func (f fixture) planTreeOf(t *testing.T, s statement) (planNode, string) {
	t.Helper()
	return f.planTreeWith(t, s, "enable_seqscan=off")
}

// planTreeWith is the plan of s with the settings given, each name=value,
// and its JSON.
func (f fixture) planTreeWith(t *testing.T, s statement, settings ...string) (planNode, string) {
	t.Helper()
	ctx := context.Background()
	var raw string
	err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
		for _, setting := range settings {
			name, value, _ := strings.Cut(setting, "=")
			if _, err := tx.Exec(ctx, `SELECT set_config($1, $2, true)`, name, value); err != nil {
				return err
			}
		}
		return tx.QueryRow(ctx, `EXPLAIN (FORMAT JSON, VERBOSE, COSTS OFF) `+s.sql, s.args...).Scan(&raw)
	})
	if err != nil {
		t.Fatal(err)
	}
	var plans []struct {
		Plan planNode `json:"Plan"`
	}
	if err := json.Unmarshal([]byte(raw), &plans); err != nil || len(plans) != 1 {
		t.Fatalf("the plan %s: %v", raw, err)
	}
	return plans[0].Plan, raw
}

// each calls visit with every node of the plan below n, n first, and
// whether it is n or below it; visit returns false not to go below a
// node.
func (n *planNode) each(visit func(*planNode) bool) {
	if !visit(n) {
		return
	}
	for i := range n.Plans {
		n.Plans[i].each(visit)
	}
}

// all are the nodes of the plan below n, n too, that match.
func (n *planNode) all(match func(*planNode) bool) []*planNode {
	var out []*planNode
	n.each(func(m *planNode) bool {
		if match(m) {
			out = append(out, m)
		}
		return true
	})
	return out
}

// outside are the nodes of the plan below root, root too, that match and
// are not below inside.
func (n *planNode) outside(inside *planNode, match func(*planNode) bool) []*planNode {
	var out []*planNode
	n.each(func(m *planNode) bool {
		if m == inside {
			return false
		}
		if match(m) {
			out = append(out, m)
		}
		return true
	})
	return out
}

// reads is a match of the scans of table, as alias when it is not "",
// by keys of index: an index scan or a bitmap of it with a condition, not
// a walk of it whole.
func reads(table, index, alias string) func(*planNode) bool {
	return func(n *planNode) bool {
		switch {
		case n.Relation != table || alias != "" && n.Alias != alias:
			return false
		case n.Index == index && n.IndexCond != "":
			return true
		}
		return n.Type == "Bitmap Heap Scan" && len(n.all(func(m *planNode) bool { return m.Index == index && m.IndexCond != "" })) > 0
	}
}

// seqScan matches a scan of a whole table but the queue's: a search reads
// its notebooks' part of it, which a scan of it all reads as fast when it
// is short, as it mostly is.
func seqScan(n *planNode) bool {
	return n.Type == "Seq Scan" && n.Relation != "page_search_queue"
}

func isSort(n *planNode) bool { return strings.Contains(n.Type, "Sort") }

// wholeSearch matches a read of page_search that reads every row, by a
// scan or a walk of an index without a condition: each would read every
// row's body, out of line (M8/P5A).
func wholeSearch(n *planNode) bool {
	return n.Relation == "page_search" && (n.Type == "Seq Scan" || strings.HasPrefix(n.Type, "Index") && n.IndexCond == "")
}

// keyed is a match of the reads of table as alias by keys of any index:
// an index scan or a bitmap with a condition, not a walk of it whole.
func keyed(table, alias string) func(*planNode) bool {
	return func(n *planNode) bool {
		if n.Relation != table || n.Alias != alias {
			return false
		}
		return n.IndexCond != "" ||
			n.Type == "Bitmap Heap Scan" && len(n.all(func(m *planNode) bool { return m.Type == "Bitmap Index Scan" && m.IndexCond != "" })) > 0
	}
}

// hits matches the read of the body tier's candidates by their lexemes
// through page_search_hits.
func hits(n *planNode) bool {
	return n.Type == "Function Scan" && n.Function == "page_search_hits"
}

// counted matches a limit straight over hits, through a subquery's scan
// at most: the hits read by their count, which the planner takes as their
// number.
func counted(n *planNode) bool {
	if n.Type != "Limit" || len(n.Plans) != 1 {
		return false
	}
	c := &n.Plans[0]
	if c.Type == "Subquery Scan" && len(c.Plans) == 1 {
		c = &c.Plans[0]
	}
	return hits(c)
}

// checkHits checks page_search_hits, which the body tier reads its
// candidates by: its settings leave its statement bitmaps of the GIN
// indexes alone, which its plan under them reads, each branch as its
// arguments take it (planned with them, and for any), for a rare key and
// a common one.
func (f fixture) checkHits(t *testing.T, stage string) {
	t.Helper()
	ctx := context.Background()
	var config []string
	var body string
	if err := f.pool.QueryRow(ctx, `SELECT proconfig, pg_get_function_sqlbody(oid) FROM pg_proc
		WHERE oid = 'page_search_hits(tsquery, tsquery)'::regprocedure`).Scan(&config, &body); err != nil {
		t.Fatal(err)
	}
	if want := []string{"enable_seqscan=off", "enable_indexscan=off", "enable_indexonlyscan=off"}; !slices.Equal(config, want) {
		t.Errorf("page_search_hits is set %q, want %q", config, want)
	}
	stmt, begun := strings.CutPrefix(strings.TrimSpace(body), "BEGIN ATOMIC")
	stmt, ended := strings.CutSuffix(strings.TrimSpace(stmt), ";\nEND")
	if !begun || !ended || !strings.Contains(stmt, "page_search_hits.body_keys") || !strings.Contains(stmt, "page_search_hits.title_keys") {
		t.Fatalf("page_search_hits is %q, want a statement of its keys", body)
	}
	stmt = strings.NewReplacer("page_search_hits.body_keys", "$1::text::tsquery", "page_search_hits.title_keys", "$2::text::tsquery").
		Replace(stmt)
	rare, common, title := "'dat' & 'ata' & 'tab' & 'aba' & 'bas' & 'ase'", "'the'", "'数'"
	for _, tt := range []struct {
		body, title *string
		indexes     []string // one of them at least
	}{
		{&rare, nil, []string{"page_search_body_idx"}},
		{&common, nil, []string{"page_search_body_idx"}},
		{&rare, &title, []string{"page_search_body_idx", "page_search_title_idx"}},
		{&common, &title, []string{"page_search_body_idx", "page_search_title_idx"}},
		{nil, &title, []string{"page_search_title_idx"}},
	} {
		s := statement{sql: stmt, args: []any{tt.body, tt.title}}
		for _, generic := range []bool{false, true} {
			settings := config
			if generic {
				settings = append(slices.Clone(config), "plan_cache_mode=force_generic_plan")
			}
			root, _ := f.planTreeWith(t, s, settings...)
			read := slices.ContainsFunc(tt.indexes, func(index string) bool {
				return len(root.all(reads("page_search", index, ""))) > 0
			})
			if !read || len(root.all(wholeSearch)) > 0 {
				t.Errorf("page_search_hits(%v, %v) %s, planned for any arguments %v, does not read %v alone", deref(tt.body), deref(tt.title),
					stage, generic, tt.indexes)
			}
		}
	}
}

// deref is *s, or "NULL".
func deref(s *string) string {
	if s == nil {
		return "NULL"
	}
	return *s
}

// The search's statements read through their indexes (M8/P5 design 3.5,
// 3.10), on a few thousand pages and ten times as many attachments, with
// their rows and statistics, whole tables' scans off but the queue's, the
// arguments bound as the store binds them; none reads every row of
// page_search, whose bodies lie out of line. The title tier reads
// page_search_title_idx and nodes by key, checks the names below its
// limit and reads the contents by key above it, sorting nothing above it.
// The body tier reads its candidates by their lexemes through
// page_search_hits, whose settings leave its statement bitmaps of the GIN
// indexes alone, by their count (a limit straight over the function);
// nodes by an index's keys, no scan of them checking the names; its walk a
// limit straight over its sort, no window below it; it reads the contents
// by key above the walk, concatenates each once, and sorts nothing outside
// the walk. The counts read the titles' and the bodies' hits so; building
// the queue's index; the job's reads and a hit's line their keys.
func TestTheSearchReadsUseTheirIndexes(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	corpus := newSearchCorpus(20261015)
	eng := corpus.pages(t, f, f.eng, 3000, 600)
	ops := corpus.pages(t, f, f.ops, 1000, 600)
	f.queueSearch(t, f.eng, false, eng...)
	f.queueSearch(t, f.ops, false, ops...)
	f.drain(t)
	f.checkHits(t, "on 4,000 rows without statistics")
	f.exec(t, "ANALYZE")
	f.checkHits(t, "on 4,000 rows")
	// Attachments, many more than pages, as a workspace's often are: the
	// tables of a size whose whole indexes no statement walks by choice.
	corpus.attachments(t, f, f.ops, 40_000)
	// A backlog of writes the job has yet to build, and an import of ops'.
	f.queueSearch(t, f.eng, false, eng[:100]...)
	f.queueSearch(t, f.ops, true, corpus.pages(t, f, f.ops, 2000, 100)...)
	f.exec(t, "ANALYZE")
	if err := f.s.AnalyzeSearch(ctx); err != nil {
		t.Fatal(err)
	}
	f.checkHits(t, "on 44,000 rows")
	notebooks := []uuid.UUID{f.eng}
	// hitsOf is the count of q's candidates by their lexemes.
	hitsOf := func(q domain.SearchQuery) int64 {
		n, err := f.s.SearchHitCount(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		return int64(n)
	}
	titles := func(q string) statement {
		sc := scopeOf(t, q, notebooks...).Query
		return statementOf(t, func(g *gen.Queries) error {
			_, err := g.SearchTitles(ctx, gen.SearchTitlesParams{Keys: sc.TitleKeys(), Notebooks: notebooks, BelowAt: now().Add(time.Hour),
				BelowID: uuid.Max(), Terms: sc.Patterns(), MaxRows: domain.DefaultSearchLimit + 1})
			return err
		})
	}
	bodies := func(q string) statement {
		sc := scopeOf(t, q, notebooks...).Query
		n := hitsOf(sc)
		return statementOf(t, func(g *gen.Queries) error {
			_, err := g.SearchBodies(ctx, gen.SearchBodiesParams{BodyKeys: sc.BodyKeys(), TitledKeys: sc.TitledKeys(), Hits: n,
				Notebooks: notebooks, BelowAt: now().Add(time.Hour), BelowID: uuid.Max(), Terms: sc.Patterns(), Titled: sc.TitledPatterns(),
				Walk: domain.MaxSearchWalk, Bodied: sc.BodiedPatterns(), MaxRows: domain.DefaultSearchLimit + 1})
			return err
		})
	}
	check := func(name string, s statement, want func(root *planNode, raw string) []string) {
		t.Helper()
		root, raw := f.planTreeOf(t, s)
		problems := want(&root, raw)
		if scans := root.all(seqScan); len(scans) > 0 {
			problems = append(problems, "scans "+scans[0].Relation+" whole")
		}
		if len(root.all(wholeSearch)) > 0 {
			problems = append(problems, "reads every row of page_search")
		}
		if len(problems) > 0 {
			t.Errorf("%s: %s:\n%s", name, strings.Join(problems, "; "), strings.Join(f.planOf(t, s), "\n"))
		}
	}
	// uses is the problems of a plan that reads none of the scans.
	uses := func(root *planNode, scans ...func(*planNode) bool) []string {
		var out []string
		for i, s := range scans {
			if len(root.all(s)) == 0 {
				out = append(out, fmt.Sprintf("does not read the index of scan %d", i+1))
			}
		}
		return out
	}
	for _, q := range []string{"database", "数据 系统", "go 电"} {
		check("SearchTitles("+q+")", titles(q), func(root *planNode, _ string) []string {
			out := uses(root, reads("page_search", "page_search_title_idx", "s"), reads("nodes", "nodes_pkey", "n"),
				reads("page_contents", "page_contents_pkey", "c2"))
			limits := root.all(func(n *planNode) bool {
				return n.Type == "Limit" && len(n.all(reads("page_search", "page_search_title_idx", "s"))) > 0
			})
			if len(limits) != 1 {
				return append(out, fmt.Sprintf("has %d limits over its candidates, want one", len(limits)))
			}
			limit := limits[0]
			if len(limit.all(func(n *planNode) bool { return strings.Contains(n.Filter+n.JoinFilter, "n.name ~ ALL") })) == 0 {
				out = append(out, "checks the names above its limit")
			}
			if len(limit.all(reads("page_contents", "page_contents_pkey", "c2"))) > 0 {
				out = append(out, "reads the contents below its limit")
			}
			if sorts := root.outside(limit, isSort); len(sorts) > 0 {
				out = append(out, "sorts above its limit")
			}
			return out
		})
	}
	for _, q := range []string{"database", "数据 系统", "latency 电", "电 #", "the"} {
		check("SearchBodies("+q+")", bodies(q), func(root *planNode, _ string) []string {
			out := uses(root, counted, keyed("nodes", "n"), reads("page_contents", "page_contents_pkey", "c2"))
			if len(root.all(func(n *planNode) bool { return n.Relation == "nodes" && strings.Contains(n.Filter, "~") })) > 0 {
				out = append(out, "checks the names in a scan of nodes")
			}
			walks := root.all(func(n *planNode) bool {
				return n.Type == "Limit" && len(n.Plans) == 1 && isSort(&n.Plans[0]) && len(n.all(hits)) > 0
			})
			if len(walks) != 1 {
				return append(out, "has no limit straight over a sort of the candidates")
			}
			walk := walks[0]
			if len(walk.all(func(n *planNode) bool { return n.Type == "WindowAgg" })) > 0 {
				out = append(out, "numbers its walk below its limit")
			}
			if len(walk.all(func(n *planNode) bool { return n.Relation == "page_contents" && n.Alias == "c2" })) > 0 {
				out = append(out, "reads the contents within its walk")
			}
			if sorts := root.outside(walk, isSort); len(sorts) > 0 {
				out = append(out, "sorts outside its walk")
			}
			// A node that computes the concatenation outputs it as itself; one
			// above it outputs what it computed, which EXPLAIN shows in
			// parentheses of its own.
			var sites []string
			computed := 0
			root.each(func(n *planNode) bool {
				for _, o := range n.Output {
					if strings.Contains(o, "c2.content ||") {
						sites = append(sites, n.Type+": "+o)
					}
					if o == "(c2.content || ''::text)" {
						computed++
					}
				}
				return true
			})
			if computed != 1 {
				out = append(out, fmt.Sprintf("concatenates a content %d times: %q", computed, sites))
			}
			return out
		})
	}
	for _, q := range []string{"database", "数据 系统", "电 #"} {
		sc := scopeOf(t, q, notebooks...).Query
		n := hitsOf(sc)
		check("SearchTitleCount("+q+")", statementOf(t, func(g *gen.Queries) error {
			_, err := g.SearchTitleCount(ctx, gen.SearchTitleCountParams{Keys: sc.TitleKeys(), Notebooks: notebooks, Terms: sc.Patterns(),
				Cap: domain.MaxSearchCount + 1})
			return err
		}), func(root *planNode, _ string) []string {
			return uses(root, reads("page_search", "page_search_title_idx", "s"), keyed("nodes", "n"))
		})
		check("SearchBodyCount("+q+")", statementOf(t, func(g *gen.Queries) error {
			_, err := g.SearchBodyCount(ctx, gen.SearchBodyCountParams{BodyKeys: sc.BodyKeys(), TitledKeys: sc.TitledKeys(), Hits: n,
				Notebooks: notebooks, Cap: domain.MaxSearchCountScan})
			return err
		}), func(root *planNode, _ string) []string {
			return uses(root, counted, keyed("nodes", "n"))
		})
		check("SearchHitCount("+q+")", statementOf(t, func(g *gen.Queries) error {
			_, err := g.SearchHitCount(ctx, gen.SearchHitCountParams{BodyKeys: sc.BodyKeys(), TitledKeys: sc.TitledKeys()})
			return err
		}), func(root *planNode, _ string) []string {
			return uses(root, hits)
		})
	}
	check("SearchBuilding", statementOf(t, func(g *gen.Queries) error {
		_, err := g.SearchBuilding(ctx, []uuid.UUID{f.eng, f.ops})
		return err
	}), func(root *planNode, _ string) []string {
		return uses(root, reads("page_search_queue", "page_search_queue_notebook_id_bulk_idx", ""))
	})
	sources := eng[:domain.SearchBatchNodes]
	for _, tt := range []struct {
		name  string
		s     statement
		scans []func(*planNode) bool
	}{
		{"QueuedSearch", statementOf(t, func(g *gen.Queries) error {
			_, err := g.QueuedSearch(ctx, gen.QueuedSearchParams{After: eng[50], MaxRows: domain.SearchBatchNodes})
			return err
		}), []func(*planNode) bool{reads("page_search_queue", "page_search_queue_pkey", "")}},
		{"SearchSources", statementOf(t, func(g *gen.Queries) error {
			_, err := g.SearchSources(ctx, sources)
			return err
		}), []func(*planNode) bool{reads("nodes", "nodes_pkey", "n"), reads("page_contents", "page_contents_pkey", "c"),
			reads("page_search", "page_search_pkey", "s")}},
		{"SearchContents", statementOf(t, func(g *gen.Queries) error {
			_, err := g.SearchContents(ctx, sources)
			return err
		}), []func(*planNode) bool{reads("page_contents", "page_contents_pkey", "")}},
		{"DeleteSearchRows", statementOf(t, func(g *gen.Queries) error {
			return g.DeleteSearchRows(ctx, sources)
		}), []func(*planNode) bool{reads("page_search", "page_search_pkey", "")}},
		{"DequeueSearch", statementOf(t, func(g *gen.Queries) error {
			return g.DequeueSearch(ctx, gen.DequeueSearchParams{Ids: sources, Versions: make([]int64, len(sources))})
		}), []func(*planNode) bool{reads("page_search_queue", "page_search_queue_pkey", "x")}},
		{"TrashLines", statementOf(t, func(g *gen.Queries) error {
			_, err := g.TrashLines(ctx, sources)
			return err
		}), []func(*planNode) bool{reads("nodes", "nodes_pkey", "n"), reads("nodes", "nodes_pkey", "p"), reads("nodes", "nodes_pkey", "nodes")}},
	} {
		check(tt.name, tt.s, func(root *planNode, _ string) []string { return uses(root, tt.scans...) })
	}
}
