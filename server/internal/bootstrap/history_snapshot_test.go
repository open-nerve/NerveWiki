package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// A log's page and a changeset's detail are read in one snapshot, as of
// the first read of the answer (M12 design 4.6.2, H5; P3 design 3.8.3,
// 13.4 item 4). Each cell's notebook has a page P renamed to Q, the
// changeset C. A holder locks page_revisions; the cell's read waits on it
// at its second read or a later one, the first (ListChangesets,
// FindChangeset, the path's StateOf) having begun, and the test waits for
// the read's wait at that later statement, by its name (waits); then a
// revert R of C is written through SQL and committed, which touches no
// page_revisions, and the holder lets go. The answer is of the moment of
// its first read: C is not reverted, and the log does not list R. Read
// again, C is reverted by R, and the log lists R: the row SQL wrote is
// read.
//
// Before the snapshot, the three logs' cells were what H5 says: C
// reverted by R in a log without R (ChangesetFacts, or the path's
// ListChangesetsOfPage, read after R). The two details' answers were one
// moment, after R; the cells hold the snapshot's moment, the first read's.
func TestTheLogReadsOneMomentAcrossARevert(t *testing.T) {
	tm := newAcmeTeam(t, "", "")
	pat := createToken(t, tm.contract, tm.base, tm.tokens["alice"])
	for i, tt := range []struct {
		name string
		rest func(nb, c string) string // REST's path; "" for MCP's tool
		tool string
		args func(c string) map[string]any
		logs bool // the answer is the whole log, which lists R or not
		// waits is the statement, sqlc's name, at which the read waits for
		// the holder: its second read or a later one, the snapshot begun.
		waits string
	}{
		{"REST's listChangesets", func(nb, _ string) string { return "/api/v0/notebooks/" + nb + "/changesets" }, "", nil, true, "ChangesetFacts"},
		{"REST's getChangeset", func(_, c string) string { return "/api/v0/changesets/" + c }, "", nil, false, "ChangesetFacts"},
		{"MCP's log", nil, "log", func(string) map[string]any { return map[string]any{} }, true, "ChangesetFacts"},
		{"MCP's log of a path", nil, "log", func(string) map[string]any { return map[string]any{"path": "Q.md"} }, false, "ListChangesetsOfPage"},
		{"MCP's changeset", nil, "changeset", func(c string) map[string]any { return map[string]any{"id": c} }, false, "ChangesetFacts"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// A notebook of each cell's own: an earlier cell's R would have
			// reverted a shared C (P3 design review B-N1).
			nb := tm.openNotebook(t, "alice", fmt.Sprintf("Moment %d", i))
			p := tm.createPage(t, "alice", nb, "", "P")
			tm.send(t, nodeRename("alice", p, "Q"), http.StatusOK)
			newest, _ := tm.restOf(t, "alice", "/api/v0/notebooks/"+nb+"/changesets?limit=1")["data"].([]any)
			if len(newest) != 1 {
				t.Fatalf("the log's newest = %v, want the rename", newest)
			}
			c, _ := newest[0].(map[string]any)["id"].(string)
			m := &mcpApp{base: tm.base, contract: tm.contract, notebook: nb, pat: pat}
			// read sends the cell's read, built here and sent on a goroutine
			// of its own; decode, on the test's, is its answer's JSON.
			read := func() (send func() any, decode func(any) map[string]any) {
				if tt.rest != nil {
					s := tm.sender(t, request("alice", http.MethodGet, tt.rest(nb, c), ""))
					return func() any { return s() }, func(got any) map[string]any {
						a := got.(answer)
						if a.err != nil || a.status != http.StatusOK {
							t.Fatalf("%s = %d %s, %v", tt.name, a.status, a.body, a.err)
						}
						tm.contract.CheckResponse(t, a.req, a.res)
						var v map[string]any
						if err := json.Unmarshal([]byte(a.body), &v); err != nil {
							t.Fatal(err)
						}
						return v
					}
				}
				return func() any { return m.use(t, tt.tool, tt.args(c)) }, func(got any) map[string]any {
					return structured(t, got.(mcpAnswer))
				}
			}

			ctx := context.Background()
			holder, err := tm.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = holder.Rollback(ctx) })
			if _, err := holder.Exec(ctx, "LOCK TABLE page_revisions IN ACCESS EXCLUSIVE MODE"); err != nil {
				t.Fatal(err)
			}
			send, decode := read()
			done := make(chan any, 1)
			go func() { done <- send() }()
			// The read's own wait, by its statement: the purge, which the
			// server runs as it starts, waits for page_revisions too, and
			// counted, R could be written before the read's first read.
			pgtest.WaitForTableLockWaitsBy(t, tm.pool, "page_revisions", tt.waits, 1, interleavingWait)
			// R, as a background revert's changeset is written: every column
			// with no default (background_cost_test.go's revertOfAttachments).
			r := uuid.NewV7().String()
			if _, err := tm.pool.Exec(ctx, `INSERT INTO changesets (id, notebook_id, kind, reverts_id, client, created_by_id, created_at, updated_at)
				SELECT $1, notebook_id, 'revert', id, 'web', created_by_id, now(), now() FROM changesets WHERE id = $2`, r, c); err != nil {
				t.Fatal(err)
			}
			if err := holder.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			var got any
			select {
			case got = <-done:
			case <-time.After(interleavingWait):
				t.Fatalf("%s did not answer once the lock was let go", tt.name)
			}
			if by, listed := revertedBy(t, decode(got), c, r); len(by) != 0 || tt.logs && listed {
				t.Errorf("%s across R = C reverted by %v, R listed %v; want the moment of its first read: neither", tt.name, by, listed)
			}
			send, decode = read()
			if by, listed := revertedBy(t, decode(send()), c, r); !slices.Equal(by, []string{r}) || tt.logs && !listed {
				t.Errorf("%s after R = C reverted by %v, R listed %v; want R, and R listed in a whole log", tt.name, by, listed)
			}
		})
	}
}

// revertedBy is what reverted the changeset c in v, a log's page (REST's
// data, MCP's changesets) or a changeset's detail (REST's, MCP's
// changeset), and whether the page lists the changeset r.
func revertedBy(t *testing.T, v map[string]any, c, r string) (by []string, listed bool) {
	t.Helper()
	var detail map[string]any
	rows, ok := v["data"].([]any)
	if !ok {
		rows, ok = v["changesets"].([]any)
	}
	switch {
	case ok:
		for _, row := range rows {
			switch row := row.(map[string]any); row["id"] {
			case c:
				detail = row
			case r:
				listed = true
			}
		}
	case v["changeset"] != nil:
		detail, _ = v["changeset"].(map[string]any)
	default:
		detail = v
	}
	if detail == nil || detail["id"] != c {
		t.Fatalf("the answer %v holds no changeset %s", v, c)
	}
	ids, _ := detail["reverted_by"].([]any)
	for _, id := range ids {
		s, _ := id.(string)
		by = append(by, s)
	}
	return by, listed
}
