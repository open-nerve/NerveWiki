package bootstrap

import (
	"net/http"
	"testing"
)

// The interleavings of the MCP batch (M9/P3 design 3.14.5; M9 design
// 4.15), harnessed as the MCP writes' (interleavings_mcp_test.go): the
// test holds a row both steps need, sends each once the one before waits,
// and lets them run in the order they came. A batch plans unlocked, then
// waits for the notebook's row in its unit; under the lock it finds the
// nodes and versions it planned on, or plans once more. Each ends by
// checking the pages' invariant, the links' and the search's.

// A web rename meets the batch at the notebook's row. The rename first:
// the batch planned on the tree before it, and under the lock finds
// another, so it plans again on the new one: its op on the path before the
// rename is page.not_found, nothing written; or, of other ops, it is done
// on the new tree, the rename's rewrite of its page kept. The batch first:
// the rename waits for it, and its rewrite reaches the page the batch
// wrote. A path that turns to another node is apply_batch_test.go's (M9/P3
// review C-M1).
func TestTheMCPBatchMeetsAWebRename(t *testing.T) {
	t.Run("the rename first, the batch's path gone", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		ids := tm.seed(t, nb, restPage{"A", "a"})
		var got mcpAnswer
		batch := tm.agent(t, "alice", nb).mcpStep(t, "alice", "batch", batched(opEdit("A.md", "a", "b")), &got)
		renamed, b := tm.interleaveOn(t, notebookRow(nb), nodeRename("bob", ids["A"], "A2"), batch)
		ran(t, b)
		if r := refusalOf(t, got); !renamed.is(http.StatusOK, "") || r.Code != "page.not_found" || refusedAt(r) != 0 ||
			tm.content(t, "alice", ids["A"]).Content != "a" {
			t.Errorf("the rename = %d %s, then the batch = %+v; want 200, then page.not_found at ops[0], A as it was", renamed.status,
				renamed.body, r)
		}
		checkInterleaved(t, tm)
	})
	t.Run("the rename first, the batch done on the new tree", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		ids := tm.seed(t, nb, restPage{"A", ""}, restPage{"P", "[[A]] p"})
		var got mcpAnswer
		batch := tm.agent(t, "alice", nb).mcpStep(t, "alice", "batch", batched(opEdit("P.md", "p", "p2")), &got)
		renamed, b := tm.interleaveOn(t, notebookRow(nb), nodeRename("bob", ids["A"], "A2"), batch)
		ran(t, b)
		done := answerOf[mcpBatched](t, got)
		if p := tm.content(t, "alice", ids["P"]); !renamed.is(http.StatusOK, "") || p.Content != "[[A2]] p2" ||
			*done.Results[0].Version != versionOf(ids["P"], 3) {
			t.Errorf("the rename = %d %s, then the batch = %+v, P %+v; want 200, then P's edit on the rename's rewrite, at 3",
				renamed.status, renamed.body, done, p)
		}
		checkInterleaved(t, tm)
	})
	t.Run("the batch first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		ids := tm.seed(t, nb, restPage{"A", ""})
		var got mcpAnswer
		batch := tm.agent(t, "alice", nb).mcpStep(t, "alice", "batch", batched(opWrite("N.md", "[[A]] n")), &got)
		b, renamed := tm.interleaveOn(t, notebookRow(nb), batch, nodeRename("bob", ids["A"], "A2"))
		ran(t, b)
		done := answerOf[mcpBatched](t, got)
		if n := tm.content(t, "alice", done.Results[0].NodeID); !renamed.is(http.StatusOK, "") || n.Content != "[[A2]] n" {
			t.Errorf("the batch = %+v, then the rename = %d %s, N %+v; want N created, then its link rewritten", done, renamed.status,
				renamed.body, n)
		}
		checkInterleaved(t, tm)
	})
}

// The batch's deferred rewrite reaches Q, linking to A it renames, while
// bob opens Q's session; the test holds Q's content row, which the opening
// and the rewrite lock after the notebook's row. The opening first: the
// batch waits for the notebook's row, then its rewrite finds Q being
// edited: linking.pages_locked at the first move, nothing written. The
// batch first: the opening waits for the notebook's row, then opens Q on
// the content the rewrite wrote.
func TestTheBatchsRewriteMeetsAnOpening(t *testing.T) {
	bothOrders(t, "the opening", "the batch", func(t *testing.T, openingFirst bool) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		ids := tm.seed(t, nb, restPage{"A", ""}, restPage{"P", "p"}, restPage{"Q", "[[A]]"})
		var got mcpAnswer
		batch := tm.agent(t, "alice", nb).mcpStep(t, "alice", "batch", batched(opEdit("P.md", "p", "p2"), opMove("A.md", "B.md")), &got)
		if openingFirst {
			opened, b := tm.interleaveBehind(t, contentRow(ids["Q"]), sessionOpening("bob", ids["Q"]), batch, "notebooks")
			ran(t, b)
			r := refusalOf(t, got)
			if !opened.is(http.StatusCreated, "") || r.Code != "linking.pages_locked" || refusedAt(r) != 1 ||
				tm.placeOf(t, ids["A"]) != "/A" || tm.content(t, "alice", ids["P"]).Content != "p" {
				t.Errorf("the opening = %d %s, then the batch = %+v; want 201, then linking.pages_locked at ops[1], nothing written",
					opened.status, opened.body, r)
			}
		} else {
			b, opened := tm.interleaveBehind(t, contentRow(ids["Q"]), batch, sessionOpening("bob", ids["Q"]), "notebooks")
			ran(t, b)
			answerOf[mcpBatched](t, got)
			if q := tm.content(t, "alice", ids["Q"]); !opened.is(http.StatusCreated, "") || q.Content != "[[B]]" || q.Revision != 2 {
				t.Errorf("the batch, then the opening = %d %s, Q %+v; want 201 on Q rewritten, at 2", opened.status, opened.body, q)
			}
		}
		checkInterleaved(t, tm)
	})
}

// A web write of P waits at P's content row, which the test holds, the
// notebook's row held for share; the batch plans on P as it was, then
// waits for the notebook's row. The write commits first: under the lock
// the batch reads P's version, another than it planned on, and plans
// again: its edit is done on the new content, old_string still in it, at
// the version after the write's. A write of the bytes P held, without a
// version, planned as no change, is planned again on the bytes the web
// wrote: page.title_taken, as the write alone answers (M9/P3 review A-M4).
func TestTheBatchIsPlannedAgainAfterAWrite(t *testing.T) {
	t.Run("an edit", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		ids := tm.seed(t, nb, restPage{"P", "alpha"})
		var got mcpAnswer
		batch := tm.agent(t, "alice", nb).mcpStep(t, "alice", "batch", batched(opEdit("P.md", "alpha", "ALPHA")), &got)
		written, b := tm.interleaveBehind(t, contentRow(ids["P"]), contentWrite("bob", ids["P"], "alpha beta", 1, ""), batch, "notebooks")
		ran(t, b)
		done := answerOf[mcpBatched](t, got)
		if p := tm.content(t, "alice", ids["P"]); !written.is(http.StatusOK, "") || p.Content != "ALPHA beta" ||
			*done.Results[0].Version != versionOf(ids["P"], 3) {
			t.Errorf("the write = %d %s, then the batch = %+v, P %+v; want 200, then the edit of the write's content, at 3",
				written.status, written.body, done, p)
		}
		checkInterleaved(t, tm)
	})
	t.Run("a write of the bytes the page held", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		ids := tm.seed(t, nb, restPage{"P", "alpha"})
		var got mcpAnswer
		batch := tm.agent(t, "alice", nb).mcpStep(t, "alice", "batch", batched(opWrite("P.md", "alpha")), &got)
		written, b := tm.interleaveBehind(t, contentRow(ids["P"]), contentWrite("bob", ids["P"], "beta", 1, ""), batch, "notebooks")
		ran(t, b)
		if r := refusalOf(t, got); !written.is(http.StatusOK, "") || r.Code != "page.title_taken" || refusedAt(r) != 0 ||
			tm.content(t, "alice", ids["P"]).Content != "beta" {
			t.Errorf("the write = %d %s, then the batch = %+v; want 200, then page.title_taken at ops[0], P the web's", written.status,
				written.body, r)
		}
		checkInterleaved(t, tm)
	})
}
