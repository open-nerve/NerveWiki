package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The index the writes keep is the index rebuilt from the pages (M6/P3
// design 5): that the links a write reaches are all it may change is
// reasoned, not proved (design 3.4), so this checks it. Runs of random
// writes through serve, in a tree whose titles repeat and clash, with
// links of every form, relative and from the root, in the body and in
// properties, and aliases: after each, in a transaction that rolls back,
// the notebook's index before its rebuild is the index after. After a
// rename or a move, each link that led to a page leads to it still, the
// links its rewrite wrote among them (M6/P4 design 8).
func TestTheIndexIsItsRebuild(t *testing.T) {
	rewritten := 0
	defer func() {
		if !t.Failed() && rewritten < 20 {
			t.Errorf("the renames and moves wrote %d pages again, want more: the runs test little of the rewrite", rewritten)
		}
	}()
	for seed := range uint64(6) {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			admin, err := reindexAdmin(tm.pool, testConfig(t, tm.url, false), slog.New(slog.NewTextHandler(io.Discard, nil)))
			if err != nil {
				t.Fatal(err)
			}
			w := writer{t: t, tm: tm, nb: tm.openNotebook(t, "alice", "Eng"), rnd: rand.New(rand.NewPCG(seed, 6))}
			written, flagged := 0, false // flagged: a link was a value of the aliases after some step
			for step := range 60 {
				led := tm.ledTo(t, w.nb)
				what, ok, relocates := w.random()
				if ok {
					written++
				}
				before, after := tm.rebuilt(t, admin, w.nb)
				if !slices.Equal(before, after) {
					t.Fatalf("after step %d, %s, the index is not its rebuild:\n%s", step, what, diff(before, after))
				}
				flagged = flagged || slices.ContainsFunc(after, func(row string) bool { return strings.HasSuffix(row, " aliases=true") })
				if now := tm.ledTo(t, w.nb); ok && relocates && !leadStill(led, now) {
					t.Fatalf("after step %d, %s, a link leads elsewhere:\n%v\nwas\n%v", step, what, now, led)
				}
			}
			if written < 30 {
				t.Errorf("serve took %d of the 60 writes, want most: the run tests little", written)
			}
			rewritten += count(t, tm.pool, `SELECT count(*) FROM page_revisions r WHERE EXISTS (SELECT 1 FROM changeset_items i
				WHERE i.changeset_id = r.changeset_id AND i.node_id <> r.node_id AND i.before_name IS NOT NULL
				AND i.after_name IS NOT NULL)`)
			if !flagged {
				t.Errorf("no link was a value of the aliases: the run tests little of the index's flag")
			}
			checkPages(t, tm.pool)
		})
	}
}

// rebuilt is the notebook nb's index, a row a line, before and after its
// rebuild, in a transaction that rolls back.
func (tm acmeTeam) rebuilt(t *testing.T, admin linking.Admin, nb string) (before, after []string) {
	t.Helper()
	errRollback := errors.New("roll back")
	err := postgres.NewTxManager(tm.pool, time.Second).WithinTx(context.Background(), func(ctx context.Context) error {
		before = tm.indexOf(ctx, t, nb)
		r, err := admin.Rebuild(ctx, uuid.MustParse(nb))
		if err != nil {
			return err
		}
		if len(r.Clashes) > 0 {
			return fmt.Errorf("the rebuild found clashes: %v", r.Clashes)
		}
		after = tm.indexOf(ctx, t, nb)
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
	return before, after
}

// indexOf is the notebook nb's index, a row a line, in order, read in the
// transaction ctx carries.
func (tm acmeTeam) indexOf(ctx context.Context, t *testing.T, nb string) []string {
	t.Helper()
	rows, err := postgres.DB(ctx, tm.pool).Query(ctx, `
		SELECT format('page %s %s %s %s', node_id, revision, extractor, frontmatter_valid) FROM indexed_pages WHERE notebook_id = $1
		UNION ALL SELECT format('link %s %s-%s %s %s %L %s %s %s %s %s %s', source_id, range_start, range_end, kind,
			coalesce(property_key, '-'), target, coalesce(anchor, '-'), coalesce(display, '-'), coalesce(target_key, '-'),
			coalesce(target_alt_key, '-'), coalesce(resolved_id::text, '-'), ambiguous) || ' aliases=' || aliases::text
		FROM page_links WHERE notebook_id = $1
		UNION ALL SELECT format('tag %s %s %s %s', source_id, tag_key, tag, count) FROM page_tags WHERE notebook_id = $1
		UNION ALL SELECT format('property %s %s %s %s', source_id, position, key, value) FROM page_properties WHERE notebook_id = $1
		UNION ALL SELECT format('alias %s %s %s', source_id, alias_key, alias) FROM page_aliases WHERE notebook_id = $1
		ORDER BY 1`, nb)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		out = append(out, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// ledTo is where the links of the notebook nb's pages resolve, by page, in
// the order written, "" for none; "alias" for a value of the aliases, which
// a rewrite leaves.
func (tm acmeTeam) ledTo(t *testing.T, nb string) map[string][]string {
	t.Helper()
	rows, err := tm.pool.Query(context.Background(), `SELECT source_id::text,
			CASE WHEN aliases THEN 'alias' ELSE coalesce(resolved_id::text, '') END
		FROM page_links WHERE notebook_id = $1 ORDER BY source_id, range_start`, nb)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var source, resolved string
		if err := rows.Scan(&source, &resolved); err != nil {
			t.Fatal(err)
		}
		out[source] = append(out[source], resolved)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// leadStill tells whether each link of was that led to a page leads to it
// in now, as many links on each page as before: a rename or move deletes
// no page, and its rewrite writes each link in its place.
func leadStill(was, now map[string][]string) bool {
	for source, links := range was {
		if len(now[source]) != len(links) {
			return false
		}
		for i, id := range links {
			if id != "" && now[source][i] != id {
				return false
			}
		}
	}
	return true
}

// diff is the lines of before alone and of after alone.
func diff(before, after []string) string {
	var b strings.Builder
	for _, l := range before {
		if !slices.Contains(after, l) {
			b.WriteString("- " + l + "\n")
		}
	}
	for _, l := range after {
		if !slices.Contains(before, l) {
			b.WriteString("+ " + l + "\n")
		}
	}
	return b.String()
}

// writer makes random writes of a notebook's pages through serve, as alice.
type writer struct {
	t   *testing.T
	tm  acmeTeam
	nb  string
	rnd *rand.Rand
}

// The titles, targets and aliases the writes draw from: titles that repeat
// and differ only by case, by "ß" or by length, a title with ".md", targets
// of every form the resolution takes, and aliases that are titles too.

func (w writer) title() string {
	return pickOf(w.rnd, "A", "B", "AB", "note", "Note.md", "dup", "Straße", "STRASSE")
}

func (w writer) target() string {
	return pickOf(w.rnd, "A", "B", "note", "dup", "A/dup", "B/note", "AB/dup", "A/B/dup", "../dup", "./note", "../../A", "/A",
		"/B/dup", "Note.md", "note.MD", "A/Note.md", "nick", "Nick", "Straße", "strasse", "missing", "A//B")
}

func (w writer) aliases() []string {
	var out []string
	for _, a := range []string{"nick", "dup", "B", "Straße"} {
		if w.rnd.IntN(2) == 0 {
			out = append(out, a)
		}
	}
	if w.rnd.IntN(2) == 0 {
		out = append(out, `"[[`+w.target()+`]]"`) // a link, which the index flags (M6/P4 fix check c4-2)
	}
	return out
}

func pickOf(rnd *rand.Rand, from ...string) string {
	return from[rnd.IntN(len(from))]
}

// livePage is a page not deleted of the notebook, with its revision.
type livePage struct {
	id       string
	revision int
}

func (w writer) pages() []livePage {
	w.t.Helper()
	rows, err := w.tm.pool.Query(context.Background(), `SELECT n.id::text, c.revision FROM nodes n
		JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL
		WHERE n.notebook_id = $1 AND n.deleted_at IS NULL ORDER BY n.id`, w.nb)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []livePage
	for rows.Next() {
		var p livePage
		if err := rows.Scan(&p.id, &p.revision); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		w.t.Fatal(err)
	}
	return out
}

// random makes one random write, and tells what it was, whether serve took
// it, and whether it was a rename or a move: one it refuses (a title
// taken, a cycle, a tree too deep) changes nothing.
func (w writer) random() (string, bool, bool) {
	w.t.Helper()
	pages := w.pages()
	pick := func() livePage { return pages[w.rnd.IntN(len(pages))] }
	parent := func() string {
		if len(pages) == 0 || w.rnd.IntN(2) == 0 {
			return ""
		}
		return pick().id
	}
	title := w.title()
	var c step
	relocates := false
	switch op := w.rnd.IntN(10); {
	case len(pages) < 4 || op < 3:
		body := map[string]any{"parent_id": nil, "title": title, "content": w.content()}
		if p := parent(); p != "" {
			body["parent_id"] = p
		}
		b, _ := json.Marshal(body)
		c = request("alice", http.MethodPost, "/api/v0/notebooks/"+w.nb+"/pages", string(b))
	case op < 5:
		c, relocates = nodeRename("alice", pick().id, title), true
	case op < 7:
		c, relocates = nodeMove("alice", pick().id, parent()), true
	case op < 8:
		c = nodeDeletion("alice", pick().id)
	default:
		p := pick()
		c = contentWrite("alice", p.id, w.content(), p.revision, "")
	}
	status, answer := ask(w.t, w.tm.contract, c.method, w.tm.base+c.path, w.tm.tokens["alice"], c.body)
	if status >= http.StatusInternalServerError {
		w.t.Fatalf("%s %s %s = %d %s", c.method, c.path, c.body, status, answer)
	}
	return fmt.Sprintf("%s %s %s = %d", c.method, c.path, c.body, status), status < http.StatusBadRequest, relocates
}

// content is a random content: links of each kind to random targets, a
// property link, aliases and a tag, each maybe.
func (w writer) content() string {
	var b strings.Builder
	if w.rnd.IntN(2) == 0 {
		b.WriteString("---\n")
		if w.rnd.IntN(2) == 0 {
			b.WriteString("aliases: [" + strings.Join(w.aliases(), ", ") + "]\n")
		}
		if w.rnd.IntN(2) == 0 {
			b.WriteString("source: \"[[" + w.target() + "]]\"\n")
		}
		b.WriteString("tags: [t]\n---\n")
	}
	for range w.rnd.IntN(5) {
		switch w.rnd.IntN(3) {
		case 0:
			b.WriteString("[[" + w.target() + "]] ")
		case 1:
			b.WriteString("![[" + w.target() + "]] ")
		default:
			b.WriteString("[x](" + w.target() + ") ")
		}
	}
	if w.rnd.IntN(3) == 0 {
		b.WriteString("#todo")
	}
	return b.String()
}
