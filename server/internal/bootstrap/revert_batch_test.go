package bootstrap

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

// The revert of each shape a batch's changeset takes, on the database
// (M9/P3 design 3.21.2): each made through MCP's batch, then reverted
// whole on the web's API, a request's revert, in rounds and with steps
// aside where it needs them.

// shapeState is the notebook nb as a revert puts it back (M9/P3 design
// 3.21.2): notebookState, each node alive at its parent, name and order
// value, and each page in the trash with its content's bytes.
func (tm acmeTeam) shapeState(t *testing.T, nb string) string {
	t.Helper()
	tree := queryStrings(t, tm.pool, `SELECT 'at ' || id || ' ' || coalesce(parent_id::text, '/') || ' ' || name || ' ' || sort_order
		FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL ORDER BY id`, nb)
	return tm.notebookState(t, nb) + "\n" + strings.Join(tree, "\n") + "\n" + strings.Join(tm.trashContents(t, nb), "\n")
}

// trashContents are the notebook nb's pages in the trash, each its id and
// its content's md5, by id.
func (tm acmeTeam) trashContents(t *testing.T, nb string) []string {
	t.Helper()
	return queryStrings(t, tm.pool, `SELECT 'trashed ' || n.id || ' ' || md5(c.content) FROM nodes n JOIN page_contents c ON c.node_id = n.id
		WHERE n.notebook_id = $1 AND n.deleted_at IS NOT NULL ORDER BY n.id`, nb)
}

// trashedNow is whether each of the nodes ids is in the trash.
func (tm acmeTeam) trashedNow(t *testing.T, ids ...string) bool {
	t.Helper()
	return count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id::text = ANY($1) AND deleted_at IS NOT NULL", ids) == len(ids)
}

// shapeBefore is the notebook nb before a changeset: its shapeState, its
// pages in the trash and every node of it.
type shapeBefore struct {
	state          string
	trashed, known []string
}

func (tm acmeTeam) shapeBefore(t *testing.T, nb string) shapeBefore {
	t.Helper()
	return shapeBefore{state: tm.shapeState(t, nb), trashed: tm.trashContents(t, nb),
		known: queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1", nb)}
}

// undoneBatch makes do's changeset in the notebook nb and reverts it
// (undoneFrom), answering the changeset and the revert's.
func (tm acmeTeam) undoneBatch(t *testing.T, nb string, do func() string) (string, string) {
	t.Helper()
	before := tm.shapeBefore(t, nb)
	c := do()
	return c, tm.undoneFrom(t, nb, before, c)
}

// undoneFrom is alice's revert of the changeset c in the notebook nb,
// whole (undoneAs). It answers the revert's changeset.
func (tm acmeTeam) undoneFrom(t *testing.T, nb string, was shapeBefore, c string) string {
	t.Helper()
	sets := tm.changesetsIn(t, nb)
	r := tm.reverted(t, "alice", c)
	tm.undoneAs(t, nb, was, c, r, sets)
	return r
}

// undoneAs checks r, a revert of the changeset c in the notebook nb of
// sets changesets before it, whole: one changeset more, r, a revert of c,
// and the notebook as before was, each page in the trash then too, of the
// same bytes, and what c made in the trash. The index is its rebuild, and
// the pages', the links', the search's and the trash's invariants hold.
func (tm acmeTeam) undoneAs(t *testing.T, nb string, was shapeBefore, c, r string, sets int) {
	t.Helper()
	if got := tm.newest(t, nb); got.ID != r || got.Kind != "revert" || got.Reverts == nil || got.Reverts.ID != c ||
		tm.changesetsIn(t, nb) != sets+1 {
		t.Errorf("the log's newest = %+v of %d changesets; want one more, the revert of %s", got, tm.changesetsIn(t, nb), c)
	}
	tm.asItWas(t, nb, was)
	tm.checkRebuilt(t, nb)
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	checkSearch(t, tm.pool)
}

// asItWas checks the notebook nb is as was, each page in the trash then
// too, of the same bytes: those in the trash now that were not, the
// changeset's own, are left out, and no node it made is alive.
func (tm acmeTeam) asItWas(t *testing.T, nb string, was shapeBefore) {
	t.Helper()
	lines := func(s string, keep func(string) bool) []string {
		return slices.DeleteFunc(strings.Split(s, "\n"), func(l string) bool { return l == "" || !keep(l) })
	}
	now := lines(tm.shapeState(t, nb), func(l string) bool { return !strings.HasPrefix(l, "trashed ") || slices.Contains(was.trashed, l) })
	if before := lines(was.state, func(string) bool { return true }); !slices.Equal(before, now) {
		t.Errorf("the notebook after the revert:\n%s\nwant as it was:\n%s", strings.Join(now, "\n"), was.state)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL AND NOT id::text = ANY($2)",
		nb, was.known); n != 0 {
		t.Errorf("%d nodes the changeset made are alive, want each in the trash", n)
	}
}

// seedAttachments makes an attachment named name under each of the
// parents in the notebook nb through SQL, its row and its file, their
// ids of one millisecond, as an upload never makes two (M9/P3 review
// C-N18): their names aside differ only in their last digits.
func (tm acmeTeam) seedAttachments(t *testing.T, nb, name string, parents ...string) []string {
	t.Helper()
	ctx := context.Background()
	store, err := storage.OpenLocal(tm.storage, 0)
	if err != nil {
		t.Fatal(err)
	}
	first := uuid.NewV7()
	var ids []string
	for i, parent := range parents {
		id := first
		if i > 0 {
			_, _ = rand.Read(id[8:])
			id[8] = 0x80 | id[8]&0x3f
		}
		blob := uuid.NewV7()
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
				SELECT $1, id, $2, 'asset', $3, lower($3), 1000, created_by_id, created_by_id, now(), now() FROM notebooks WHERE id = $4`,
				[]any{id, parent, name, nb}},
			{`INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at)
				SELECT $1, n.id, n.notebook_id, 'image/png', 1, sha256('x'), n.created_by_id, now() FROM nodes n WHERE n.id = $2`,
				[]any{blob, id}},
			{`INSERT INTO page_search_queue (node_id, notebook_id, bulk) VALUES ($1, $2, false)`, []any{id, nb}},
		} {
			if _, err := tm.pool.Exec(ctx, q.sql, q.args...); err != nil {
				t.Fatalf("%s: %v", q.sql, err)
			}
		}
		w, err := store.Create(ctx, "blobs/"+blob.String())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
		if err := w.Commit(); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id.String())
	}
	if a, b := ids[0][:13], ids[len(ids)-1][:13]; a != b || ids[0] == ids[len(ids)-1] {
		t.Fatalf("the attachments' ids %v, want them of one millisecond", ids)
	}
	return ids
}

// Each shape of 4.12 a batch's changeset takes is reverted whole through
// the web's request, the notebook as before it: the tree, each node's
// parent, name and order value, each page's bytes and those of each page
// in the trash, the index its rebuild, and the pages', links', search's
// and trash's invariants; one changeset more, a revert. Names swapped
// through a third, put back with a step aside that no item of the revert
// keeps; a rename, then a page of the old name; a deletion, then a page of
// its name; a move into a folder made; a move and rename past a sibling
// holding the name it has between; the review's counterexample; long
// names swapped and two attachments of one name, of one millisecond, each
// moved out of a folder deleted and made again, then back in, their names
// aside within a title's bytes and apart; a page written, then deleted,
// restored and written back to its content before, and that revert's
// revert, which writes it again and deletes it, its content in the trash
// the batch's. Two trash items one under the other no batch nor revert
// makes: the changeset seeded so, the revert reads the two deletions as
// one group, which the trash does not hold, and refuses it as changed,
// writing nothing (TestARestoreWaitsForTheGroupItGoesUnder plans their
// schedule).
func TestTheRevertOfEachBatchShape(t *testing.T) {
	tm, logs := newAcmeTeamLogged(t)
	agent := tm.agent(t, "alice", "")
	notebooks := 0
	// own is the log lines of the subtest running, from its start (fix
	// round 1, C-N3): one's error fails it alone.
	var own func() *syncBuffer
	each := func(name string, pages []restPage, test func(t *testing.T, nb string, ids map[string]string, m *mcpApp)) {
		t.Run(name, func(t *testing.T) {
			own = logsFrom(logs)
			notebooks++
			nb := tm.openNotebook(t, "alice", fmt.Sprint("Eng ", notebooks))
			test(t, nb, tm.seed(t, nb, pages...), agent.in(nb))
			noErrorLogged(t, own())
		})
	}
	batch := func(t *testing.T, m *mcpApp, ops ...map[string]any) func() string {
		return func() string {
			t.Helper()
			b := m.batchDone(t, ops...)
			if b.ChangesetID == nil {
				t.Fatalf("the batch = %+v, want a changeset", b)
			}
			return *b.ChangesetID
		}
	}
	at := func(t *testing.T, id, want string) {
		t.Helper()
		if got := tm.placeOf(t, id); got != want {
			t.Errorf("the node %s at %s, want %s", id, got, want)
		}
	}

	each("names swapped through a third", []restPage{{"A", "a"}, {"B", "b"}, {"Linker", "[[A]] [[B]]"}},
		func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
			_, r := tm.undoneBatch(t, nb, batch(t, m, opMove("A.md", "T.md"), opMove("B.md", "A.md"), opMove("T.md", "B.md")))
			at(t, ids["A"], "/A")
			at(t, ids["B"], "/B")
			if n := count(t, tm.pool, `SELECT count(*) FROM changeset_items WHERE changeset_id = $1
				AND (before_name LIKE '% ~%' OR after_name LIKE '% ~%')`, r); n != 0 {
				t.Errorf("%d of the revert's items name a node aside, want none: merged, the first before and the last after", n)
			}
		})
	each("a rename, then a page of the old name", []restPage{{"Plan", "plan"}, {"Linker", "[[Plan]] [[Old]]"}},
		func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
			c, _ := tm.undoneBatch(t, nb, batch(t, m, opMove("Plan.md", "Old.md"), opWrite("Plan.md", "new plan")))
			made := queryStrings(t, tm.pool, "SELECT node_id::text FROM changeset_items WHERE changeset_id = $1 AND before_name IS NULL", c)
			if len(made) != 1 || !tm.trashedNow(t, made...) {
				t.Errorf("the batch made %v, want the new Plan, in the trash", made)
			}
			at(t, ids["Plan"], "/Plan")
		})
	each("a deletion, then a page of its name", []restPage{{"X", "x"}}, func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
		c, _ := tm.undoneBatch(t, nb, batch(t, m, opDelete("X.md"), opWrite("X.md", "new x")))
		made := queryStrings(t, tm.pool, "SELECT node_id::text FROM changeset_items WHERE changeset_id = $1 AND before_name IS NULL", c)
		if len(made) != 1 || !tm.trashedNow(t, made...) {
			t.Errorf("the batch made %v, want the new X, in the trash", made)
		}
		at(t, ids["X"], "/X")
	})
	each("a move into a folder made", []restPage{{"X", "x"}}, func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
		c, _ := tm.undoneBatch(t, nb, batch(t, m, opWrite("F.md", "f"), opMove("X.md", "F/X.md")))
		made := queryStrings(t, tm.pool, "SELECT node_id::text FROM changeset_items WHERE changeset_id = $1 AND before_name IS NULL", c)
		if len(made) != 1 || !tm.trashedNow(t, made...) {
			t.Errorf("the batch made %v, want F, in the trash", made)
		}
		at(t, ids["X"], "/X")
	})
	each("a move and rename past a sibling holding the name between", []restPage{{"F", ""}, {"F/A", "a"}, {"F/B", "b"}, {"G", ""}},
		func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
			tm.undoneBatch(t, nb, batch(t, m, opMove("F/B.md", "G/B.md"), opMove("G/B.md", "G/A.md")))
			at(t, ids["F/B"], ids["F"]+"/B")
			at(t, ids["F/A"], ids["F"]+"/A")
		})
	each("the review's counterexample", []restPage{{"G", "g"}, {"G/x", "x"}, {"G/y", "y"}},
		func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
			c, _ := tm.undoneBatch(t, nb, batch(t, m, opMove("G/x.md", "x.md"), opMove("G/y.md", "y.md"), opDelete("G.md"),
				opWrite("G.md", "new g"), opMove("y.md", "G/y.md")))
			made := queryStrings(t, tm.pool, "SELECT node_id::text FROM changeset_items WHERE changeset_id = $1 AND before_name IS NULL", c)
			if len(made) != 1 || !tm.trashedNow(t, made...) {
				t.Errorf("the batch made %v, want the new G, in the trash", made)
			}
			at(t, ids["G"], "/G")
			at(t, ids["G/x"], ids["G"]+"/x")
			at(t, ids["G/y"], ids["G"]+"/y")
		})
	long, longer := strings.Repeat("名", 82), strings.Repeat("名", 85) // 246 and 255 bytes
	each("long names swapped, two attachments of one name and one millisecond moved aside",
		[]restPage{{long, "l"}, {longer, "m"}, {"F1", ""}, {"F2", ""}},
		func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
			photos := tm.seedAttachments(t, nb, "photo.png", ids["F1"], ids["F2"])
			ops := []map[string]any{opMove(long+".md", "T.md"), opMove(longer+".md", long+".md"), opMove("T.md", longer+".md")}
			for i, f := range []string{"F1", "F2"} {
				out := fmt.Sprint("out", i, ".png")
				ops = append(ops, opMove(f+"/photo.png", out), opDelete(f+".md"), opWrite(f+".md", "new"), opMove(out, f+"/photo.png"))
			}
			tm.undoneBatch(t, nb, batch(t, m, ops...))
			at(t, ids[long], "/"+long)
			at(t, ids[longer], "/"+longer)
			at(t, photos[0], ids["F1"]+"/photo.png")
			at(t, photos[1], ids["F2"]+"/photo.png")
			if lines := (&mcpApp{logs: own()}).loggedAt(t, "INFO"); !slices.ContainsFunc(lines, func(e map[string]any) bool {
				return e["msg"] == "changeset reverted" && e["aside"] == float64(3)
			}) {
				t.Errorf("no revert logged three steps aside: a name renamed aside and two attachments moved aside")
			}
		})
	var written, writtenRevert, writtenNB string
	var deleted shapeBefore // the notebook after the batch
	each("a page written, then deleted", []restPage{{"X", "before"}}, func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
		before := tm.shapeBefore(t, nb)
		c := batch(t, m, opEdit("X.md", "before", "edited"), opDelete("X.md"))()
		deleted = tm.shapeBefore(t, nb)
		writtenRevert = tm.undoneFrom(t, nb, before, c)
		if got := tm.content(t, "alice", ids["X"]).Content; got != "before" {
			t.Errorf("X restored holds %q, want its content before the batch", got)
		}
		written, writtenNB = ids["X"], nb
	})
	t.Run("that revert's revert", func(t *testing.T) {
		own = logsFrom(logs)
		if writtenRevert == "" {
			t.Skip("the revert it reverts failed")
		}
		tm.undoneFrom(t, writtenNB, deleted, writtenRevert)
		if !tm.trashedNow(t, written) || queryStrings(t, tm.pool, "SELECT content FROM page_contents WHERE node_id = $1", written)[0] != "edited" {
			t.Errorf("X is not in the trash with the content the batch deleted")
		}
		noErrorLogged(t, own())
	})
	each("two trash items one under the other, seeded", []restPage{{"A", "a"}, {"A/X", "x"}, {"A/X/B", "b"}},
		func(t *testing.T, nb string, ids map[string]string, m *mcpApp) {
			tm.send(t, nodeDeletion("alice", ids["A/X/B"]), http.StatusNoContent)
			inner := tm.newest(t, nb).ID
			tm.send(t, nodeDeletion("alice", ids["A"]), http.StatusNoContent)
			outer := tm.newest(t, nb).ID
			for _, q := range []struct {
				sql  string
				args []any
			}{
				{"UPDATE changeset_items SET changeset_id = $1 WHERE changeset_id = $2", []any{outer, inner}},
				{"DELETE FROM changesets WHERE id = $1", []any{inner}},
			} {
				if _, err := tm.pool.Exec(context.Background(), q.sql, q.args...); err != nil {
					t.Fatalf("%s: %v", q.sql, err)
				}
			}
			was, sets := tm.shapeState(t, nb), tm.changesetsIn(t, nb)
			c := changesetRevert("alice", outer)
			status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["alice"], c.body)
			a := answer{status: status, body: body, code: problemCode(t, body)}
			conflicts, n := conflictsOf(t, a)
			reasons := map[string]string{}
			for _, x := range conflicts {
				reasons[x.NodeID] = x.Reason
			}
			if !a.is(http.StatusConflict, "page.revert_conflict") || n != 2 || reasons[ids["A"]] != "changed" || reasons[ids["A/X/B"]] != "changed" {
				t.Errorf("the revert = %d %s; want 409 page.revert_conflict, A and B changed", status, body)
			}
			if tm.shapeState(t, nb) != was || tm.changesetsIn(t, nb) != sets {
				t.Errorf("the revert refused changed the notebook")
			}
			tm.checkHistory(t)
		})
}

// logsFrom is the lines logs holds from now on, each time it is called,
// as a buffer of their own.
func logsFrom(logs *syncBuffer) func() *syncBuffer {
	from := len(logs.String())
	return func() *syncBuffer {
		out := &syncBuffer{}
		_, _ = out.Write([]byte(logs.String()[from:]))
		return out
	}
}

// The revert of a lone page a batch wrote and then deleted, both its
// contents near the largest, restores the one and writes the other back,
// two contents of the page parsed (M9/P3 design 3.18.4): at the least
// parse budget the configuration takes, a page's largest content, the
// second's take cannot fit beside the first's share, which the revert
// gives back first (fix round 1, B-M1). The default budget's case, both
// shares kept, is TestDecidedAllGivesItsSharesBackBeforeATakeThatCannotFit's:
// a row of it here could fail under neither mutant of the give-back (fix
// round 2, C's N2). The app's deadlines are production's, as
// TestTheMCPBatchIsRefusedWhole's are: the batch that writes the 5 MiB
// content takes some 2.5 s under the race detector here, past the tests'
// 4 s on CI (run 39481193146).
func TestTheRevertOfALonePagesTwoContentsAtTheLeastBudget(t *testing.T) {
	size := domain.MaxContentBytes - 64
	fill := func(tag string) string { return (tag + "\n" + strings.Repeat("a", size))[:size] }
	tm := newAcmeTeamWith(t, "member", "", func(c *config.Config) {
		c.Server.WriteTimeout, c.Server.RequestTimeout = 60*time.Second, 15*time.Second
		c.Page.ParseBudgetBytes = config.MinParseBudgetBytes
	})
	nb := tm.openNotebook(t, "alice", "Eng")
	before, written := fill("before"), fill("written")
	ids := tm.seed(t, nb, restPage{"X", before})
	b := tm.agent(t, "alice", nb).batchDone(t, opWriteOn("X.md", written, versionOf(ids["X"], 1)), opDelete("X.md"))
	if b.ChangesetID == nil {
		t.Fatalf("the batch = %+v, want a changeset", b)
	}
	c := changesetRevert("alice", *b.ChangesetID)
	started := time.Now()
	status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["alice"], c.body)
	t.Logf("the revert answered in %v", time.Since(started))
	if status != http.StatusOK {
		t.Fatalf("the revert = %d %.300s, want 200", status, body)
	}
	if got := tm.content(t, "alice", ids["X"]); got.Content != before {
		t.Errorf("X = revision %d, %d bytes; want the content before", got.Revision, len(got.Content))
	}
}
