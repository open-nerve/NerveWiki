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

func TestTheSourceUploadsAndDowngradesSerialize(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("uploads/reverse_%t", reverse), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			wiki := tm.llmWiki(t, "Two uploaded sources")
			before := len(storedBlobs(t, tm.storage))
			a := assetUpload(t, "alice", wiki.id, wiki.raw, "a.bin", "equal source attachment bytes")
			b := assetUpload(t, "bob", wiki.id, wiki.raw, "b.bin", "equal source attachment bytes")
			if reverse {
				a, b = b, a
			}
			first, second := tm.interleaveWith(t, notebookRow(wiki.id).lock, func(i int) {
				pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", i+1, interleavingWait)
				if files := storedBlobs(t, tm.storage); len(files) != before+i+1 {
					t.Fatalf("upload %d waits for notebook with %d files, want %d completed Puts", i, len(files), before+i+1)
				}
			}, a, b)
			if first.status != http.StatusCreated || !second.is(http.StatusConflict, "page.source_duplicate") {
				t.Errorf("upload order = %d %s then %d %s", first.status, first.body, second.status, second.body)
			}
			winner := idOf(t, first)
			var refused struct {
				DuplicateOf *string `json:"duplicate_of"`
			}
			decodeAnswer(t, second.body, &refused)
			if refused.DuplicateOf == nil || *refused.DuplicateOf != winner {
				t.Errorf("losing upload target = %v, want committed %s", refused.DuplicateOf, winner)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM asset_blobs WHERE notebook_id=$1", wiki.id); n != 1 {
				t.Errorf("two uploads persisted %d blobs, want one", n)
			}
			if files := storedBlobs(t, tm.storage); len(files) != before+1 {
				t.Errorf("losing upload left %d files, want %d", len(files), before+1)
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
			checkAssets(t, tm.pool, tm.storage)
		})
	}
	for _, downgradeFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("first_write/downgrade_first_%t", downgradeFirst), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			wiki := tm.llmWiki(t, "Downgrade versus first source")
			const content = "already committed source body"
			known := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Known", content)
			empty := tm.createPage(t, "bob", wiki.id, wiki.raw, "Empty")
			write := contentWrite("bob", empty, content, 1, "")
			downgrade := request("alice", http.MethodPost, "/api/v0/notebooks/"+wiki.id+"/downgrade", "")
			key := page.SourceLockKey(uuid.MustParse(wiki.id))
			var written, downgraded answer
			if downgradeFirst {
				// Keep the source advisory held even after the downgrade. A
				// writer that reads standard mode must not acquire it at all.
				holder, err := tm.pool.Begin(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = holder.Rollback(context.Background()) }()
				if _, err := holder.Exec(t.Context(), "SELECT pg_advisory_xact_lock($1,$2)", int32(0x73726365), key); err != nil {
					t.Fatal(err)
				}
				downgraded, written = tm.interleaveOn(t, notebookRow(wiki.id), downgrade, write)
				if !written.is(http.StatusOK, "") {
					t.Errorf("first write after downgrade = %d %s, want ordinary success despite held source lock", written.status, written.body)
				}
				tm.wrote(t, empty, content, 2)
			} else {
				// The first writer already owns notebook SHARE and waits at
				// the source advisory. The downgrade is observed waiting for
				// its notebook lock before the advisory holder releases.
				written, downgraded = tm.interleaveWith(t, fmt.Sprintf("SELECT pg_advisory_xact_lock(%d,%d)", 0x73726365, key), func(i int) {
					if i == 0 {
						pgtest.WaitForAdvisoryLockWaits(t, tm.pool, 0x73726365, key, 1, interleavingWait)
					} else {
						pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
					}
				}, write, downgrade)
				var refused struct {
					DuplicateOf *string `json:"duplicate_of"`
				}
				decodeAnswer(t, written.body, &refused)
				if !written.is(http.StatusConflict, "page.source_duplicate") || refused.DuplicateOf == nil || *refused.DuplicateOf != known {
					t.Errorf("first write before downgrade = %d %s, want duplicate target %s", written.status, written.body, known)
				}
				tm.wrote(t, empty, "", 1)
			}
			if !downgraded.is(http.StatusOK, "") {
				t.Errorf("downgrade = %d %s", downgraded.status, downgraded.body)
			}
			if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND source_hash IS NOT NULL", wiki.id) != 0 {
				t.Error("downgraded first-write order retained source hashes")
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
		t.Run(fmt.Sprintf("session/downgrade_first_%t", downgradeFirst), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			wiki := tm.llmWiki(t, "Downgrade versus source session")
			source := tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Source", "unchanged source body")
			before := tm.content(t, "bob", source)
			sets := tm.changesetsIn(t, wiki.id)
			downgrade := request("alice", http.MethodPost, "/api/v0/notebooks/"+wiki.id+"/downgrade", "")
			open := sessionOpening("bob", source)
			var opened, downgraded answer
			if downgradeFirst {
				downgraded, opened = tm.interleaveOn(t, notebookRow(wiki.id), downgrade, open)
				if !opened.is(http.StatusCreated, "") || tm.sessionsOf(t, source) != 1 {
					t.Errorf("session after downgrade = %d %s, want one real session", opened.status, opened.body)
				}
			} else {
				opened, downgraded = tm.interleaveOn(t, notebookRow(wiki.id), open, downgrade)
				if !opened.is(http.StatusConflict, "page.source_immutable") || tm.sessionsOf(t, source) != 0 {
					t.Errorf("session before downgrade = %d %s, want immutable and no session", opened.status, opened.body)
				}
			}
			if !downgraded.is(http.StatusOK, "") {
				t.Errorf("downgrade beside session = %d %s", downgraded.status, downgraded.body)
			}
			if after := tm.content(t, "bob", source); after != before || tm.changesetsIn(t, wiki.id) != sets {
				t.Errorf("session/downgrade changed source content or history: before=%+v after=%+v", before, after)
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	}
}
