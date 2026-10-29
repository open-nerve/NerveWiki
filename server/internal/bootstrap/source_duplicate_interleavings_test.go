package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The two first writes already hold distinct content rows and SHARE on
// the notebook. Both are observed at the actual source advisory lock before
// either may proceed; sleeps or presumed request arrival order are not used.
func TestTheSourceWritersSerializeInBothLockModes(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("two_first_writes/reverse_%t", reverse), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			wiki := tm.llmWiki(t, "Concurrent first sources")
			first := tm.createPage(t, "alice", wiki.id, wiki.raw, "A")
			second := tm.createPage(t, "bob", wiki.id, wiki.raw, "B")
			if reverse {
				first, second = second, first
			}
			before := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", wiki.id)
			key := page.SourceLockKey(uuid.MustParse(wiki.id))
			a, b := tm.interleaveWith(t, fmt.Sprintf("SELECT pg_advisory_xact_lock(%d,%d)", 0x73726365, key), func(i int) {
				pgtest.WaitForAdvisoryLockWaits(t, tm.pool, 0x73726365, key, i+1, interleavingWait)
			}, contentWrite("alice", first, "one exact source body", 1, ""), contentWrite("bob", second, "one exact source body", 1, ""))
			if !a.is(http.StatusOK, "") || !b.is(http.StatusConflict, "page.source_duplicate") {
				t.Errorf("first source writes = %d %s / %d %s, want success then duplicate", a.status, a.body, b.status, b.body)
			}
			var problem struct {
				DuplicateOf *string `json:"duplicate_of"`
			}
			decodeAnswer(t, b.body, &problem)
			if problem.DuplicateOf == nil || *problem.DuplicateOf != first {
				t.Errorf("loser's target=%v, want committed first %s", problem.DuplicateOf, first)
			}
			tm.wrote(t, first, "one exact source body", 2)
			tm.wrote(t, second, "", 1)
			if n := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id=$1", wiki.id); n != before+1 {
				t.Errorf("first writes added %d changesets, want one", n-before)
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
		})
	}
	for _, treeFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("tree_and_first/tree_first_%t", treeFirst), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			wiki := tm.llmWiki(t, "Tree versus first source")
			empty := tm.createPage(t, "alice", wiki.id, wiki.raw, "Empty")
			create := request("alice", http.MethodPost, "/api/v0/notebooks/"+wiki.id+"/pages",
				fmt.Sprintf(`{"parent_id":%q,"title":"Created","content":"same body"}`, wiki.raw))
			write := contentWrite("bob", empty, "same body", 1, "")
			first, second := write, create
			if treeFirst {
				first, second = create, write
			}
			a, b := tm.interleaveOn(t, notebookRow(wiki.id), first, second)
			wantStatus, winner := http.StatusOK, empty
			if treeFirst {
				wantStatus = http.StatusCreated
				winner = idOf(t, a)
			}
			if a.status != wantStatus || !b.is(http.StatusConflict, "page.source_duplicate") {
				t.Errorf("tree/SHARE order = %d %s then %d %s", a.status, a.body, b.status, b.body)
			}
			var problem struct {
				DuplicateOf *string `json:"duplicate_of"`
			}
			decodeAnswer(t, b.body, &problem)
			if problem.DuplicateOf == nil || *problem.DuplicateOf != winner {
				t.Errorf("tree/SHARE loser target=%v, want %s", problem.DuplicateOf, winner)
			}
			if treeFirst {
				tm.wrote(t, empty, "", 1)
				tm.wrote(t, winner, "same body", 1)
			} else {
				tm.wrote(t, empty, "same body", 2)
				if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND name='Created'", wiki.id) != 0 {
					t.Error("losing tree write left a created node")
				}
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
		})
	}
	t.Run("tree_does_not_take_source_advisory", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		wiki := tm.llmWiki(t, "Tree source lock")
		holder, err := tm.pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = holder.Rollback(context.Background()) }()
		if _, err := holder.Exec(t.Context(), "SELECT pg_advisory_xact_lock($1,$2)", int32(0x73726365), page.SourceLockKey(uuid.MustParse(wiki.id))); err != nil {
			t.Fatal(err)
		}
		id := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Created", "tree source bytes")
		tm.wrote(t, id, "tree source bytes", 1)
		assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
		checkPages(t, tm.pool)
		checkLinks(t, tm.pool)
	})
}
