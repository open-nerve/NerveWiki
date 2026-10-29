package bootstrap

import (
	"crypto/sha256"
	"net/http"
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// A source's link rewrite and both reverts keep their actual versions
// through the composition root (M10/P2 design 3.4). REST's rename uses
// immediate participants; MCP's batch move uses deferred participants.
// The background reverts commit the two source writes in separate batches.
func TestTheSourceRewriteSurvivesUndoAndUndoAgain(t *testing.T) {
	tm := newAcmeTeamWired(t, "member", "", nil, transfer.Batch{Nodes: 1})
	for _, mode := range []string{"sync", "background"} {
		t.Run(mode, func(t *testing.T) {
			wiki := tm.llmWiki(t, "Rewrite "+mode)
			raw := tm.content(t, "alice", wiki.raw)
			agents := tm.content(t, "alice", wiki.agents)
			target := tm.createPage(t, "alice", wiki.id, "", "Target")
			targetCreated := tm.newest(t, wiki.id).ID
			sources := []struct {
				name, before, after string
				id, created         string
			}{
				{name: "One", before: "一 [[Target]]\n", after: "一 [[LongerTarget]]\n"},
				{name: "Two", before: "第二条 [[Target]]\n", after: "第二条 [[LongerTarget]]\n"},
			}
			for i := range sources {
				s := &sources[i]
				s.id = tm.createPageWith(t, "alice", wiki.id, wiki.raw, s.name, s.before)
				s.created = tm.newest(t, wiki.id).ID
				tm.wrote(t, s.id, s.before, 1)
				tm.sourceRewriteVersion(t, s.created, s.id, s.before, 0, 1, false)
			}

			// Only the sources gain body versions. A tree-only target
			// rename and both reverts leave its first body version alone.
			unchanged := func() {
				t.Helper()
				assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
				for _, p := range []struct {
					id, created, body string
				}{
					{target, targetCreated, ""},
					{wiki.raw, wiki.template, raw.Content},
					{wiki.agents, wiki.template, agents.Content},
				} {
					tm.wrote(t, p.id, p.body, 1)
					tm.sourceRewriteVersion(t, p.created, p.id, p.body, 0, 1, false)
					if n := count(t, tm.pool, "SELECT count(*) FROM page_revisions WHERE node_id = $1", p.id); n != 1 {
						t.Errorf("unchanged page %s has %d body versions, want 1", p.id, n)
					}
				}
				if n := count(t, tm.pool, `SELECT count(*) FROM nodes WHERE parent_id IS NULL AND deleted_at IS NULL
					AND ((id = $1 AND name = 'raw') OR (id = $2 AND name = 'AGENTS'))`, wiki.raw, wiki.agents); n != 2 {
					t.Errorf("the template keeps %d of its root nodes in place, want 2", n)
				}
			}
			unchanged()

			check := func(changeset string, revision int, bodyRewritten, rewriteMarker bool) {
				t.Helper()
				for _, s := range sources {
					body := s.before
					if bodyRewritten {
						body = s.after
					}
					tm.wrote(t, s.id, body, revision)
					tm.sourceRewriteVersion(t, changeset, s.id, body, revision-1, revision, rewriteMarker)
					if n := count(t, tm.pool, "SELECT count(*) FROM page_revisions WHERE node_id = $1", s.id); n != revision {
						t.Errorf("source %s has %d body versions, want %d", s.name, n, revision)
					}
					tm.resolves(t, s.id, target)
				}
				name := "Target"
				if bodyRewritten {
					name = "LongerTarget"
				}
				if n := count(t, tm.pool, `SELECT count(*) FROM nodes
					WHERE id = $1 AND name = $2 AND parent_id IS NULL AND deleted_at IS NULL`, target, name); n != 1 {
					t.Errorf("the target at revision %d is not named %s at the root", revision, name)
				}
				unchanged()
			}

			var changed string
			if mode == "sync" {
				tm.send(t, nodeRename("alice", target, "LongerTarget"), http.StatusOK)
				changed = tm.newest(t, wiki.id).ID
			} else {
				b := tm.agent(t, "alice", wiki.id).batchDone(t, opWith(opMove("Target.md", "LongerTarget.md"), "update_links", true))
				if b.ChangesetID == nil {
					t.Fatalf("the batch = %+v, want its changeset", b)
				}
				changed = *b.ChangesetID
			}
			check(changed, 2, true, true)

			revert := func(changeset string) string {
				t.Helper()
				if mode == "sync" {
					return tm.reverted(t, "alice", changeset)
				}
				return tm.sourceRewriteBackgroundRevert(t, wiki.id, changeset, target, sources[0].id, sources[1].id)
			}
			undone := revert(changed)
			// Undo restores the original body but remains a system rewrite.
			check(undone, 3, false, true)
			again := revert(undone)
			check(again, 4, true, true)
			if changed == undone || changed == again || undone == again {
				t.Errorf("rewrite, undo and undo-of-undo changesets = %q, %q, %q, want distinct ids", changed, undone, again)
			}
			for _, pair := range [][2]string{{undone, changed}, {again, undone}} {
				if n := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE id = $1 AND reverts_id = $2", pair[0], pair[1]); n != 1 {
					t.Errorf("changeset %s does not record its revert of %s", pair[0], pair[1])
				}
			}
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
			if mode == "background" {
				checkBackground(t, tm)
			}
		})
	}
}

// sourceRewriteVersion observes exactly one persisted version from the
// public operation, including its marker, base and bytes actually written.
// A zero base means creation, whose base_revision must be NULL.
func (tm acmeTeam) sourceRewriteVersion(t *testing.T, changeset, id, content string, base, revision int, rewrite bool) {
	t.Helper()
	if n := count(t, tm.pool, "SELECT count(*) FROM page_revisions WHERE changeset_id = $1 AND node_id = $2", changeset, id); n != 1 {
		t.Fatalf("changeset %s has %d versions for page %s, want 1", changeset, n, id)
	}
	var gotBase *int
	var gotRevision, gotBytes int
	var gotContent string
	var gotHash []byte
	var gotRewrite bool
	if err := tm.pool.QueryRow(t.Context(), `SELECT r.base_revision, r.revision, r.content, r.content_hash, r.byte_size, r.rewrite
		FROM page_revisions r WHERE r.changeset_id = $1 AND r.node_id = $2`, changeset, id).
		Scan(&gotBase, &gotRevision, &gotContent, &gotHash, &gotBytes, &gotRewrite); err != nil {
		t.Fatal(err)
	}
	if gotRewrite != rewrite {
		t.Errorf("page %s in changeset %s at revision %d: rewrite = %t, want %t", id, changeset, gotRevision, gotRewrite, rewrite)
	}
	if base == 0 {
		if gotBase != nil {
			t.Errorf("page %s's creation has base %d, want NULL", id, *gotBase)
		}
	} else if gotBase == nil || *gotBase != base {
		t.Errorf("page %s in changeset %s has base %v, want %d", id, changeset, gotBase, base)
	}
	wantHash := sha256.Sum256([]byte(content))
	if gotRevision != revision || gotContent != content || gotBytes != len(content) || !slices.Equal(gotHash, wantHash[:]) {
		t.Errorf("page %s in changeset %s: revision %d, body %q, hash %x, bytes %d; want %d, %q, %x, %d",
			id, changeset, gotRevision, gotContent, gotHash, gotBytes, revision, content, wantHash, len(content))
	}
}

// sourceRewriteBackgroundRevert releases actual node locks one at a time.
// Seeing one source's committed version while the next batch waits proves
// the compact plan and writer are exercised across transaction boundaries.
func (tm acmeTeam) sourceRewriteBackgroundRevert(t *testing.T, nb, changeset, target, one, two string) string {
	t.Helper()
	h := tm.holdNodes(t, target, one, two)
	j := tm.startedJob(t, revertStart("alice", nb, changeset, ""))
	var seen []int
	h.inTurn(t, func() {
		seen = append(seen, count(t, tm.pool, `SELECT count(*) FROM page_revisions
			WHERE changeset_id = $1 AND node_id IN ($2, $3)`, *j.ChangesetID, one, two))
	})
	got := tm.endedBackground(t, "alice", j.ID)
	if got.State != "succeeded" || got.Progress.Done != 3 || got.counts()["written"] != 2 || got.counts()["put_back"] != 1 {
		t.Fatalf("the source rewrite's revert = %s, want succeeded, done 3, written 2 and put_back 1", got)
	}
	if got.ChangesetID == nil || *got.ChangesetID != *j.ChangesetID {
		t.Fatalf("the finished revert = %s, want the started changeset %s", got, *j.ChangesetID)
	}
	if !slices.Contains(seen, 1) {
		t.Errorf("source versions committed while a later batch waited: %v, want to observe exactly 1", seen)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM page_revisions
		WHERE changeset_id = $1 AND node_id IN ($2, $3)`, *got.ChangesetID, one, two); n != 2 {
		t.Errorf("the finished revert has %d source versions, want 2", n)
	}
	t.Logf("revert %s: committed source versions at held nodes = %v, final = 2", *got.ChangesetID, seen)
	return *got.ChangesetID
}
