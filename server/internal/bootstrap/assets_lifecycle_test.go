package bootstrap

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// pngFile is a file the server takes for a PNG: its signature.
const pngFile = "\x89PNG\r\n\x1a\n"

// An attachment's row follows its node (M7/P2 design 3.8): deleted alone,
// or with a subtree, at the node's time; the attachment reads 404, and so
// does its address. One elsewhere stays.
func TestDeletingANodeDeletesItsAttachmentsRows(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	root := tm.createPage(t, "alice", nb, "", "Root")
	child := tm.createPage(t, "alice", nb, root, "Child")
	underChild := tm.upload(t, "bob", nb, child, "a.png", pngFile)
	tm.upload(t, "bob", nb, root, "b.pdf", "%PDF-1.4")
	alone := tm.upload(t, "alice", nb, root, "d.txt", "deleted alone")
	kept := tm.upload(t, "alice", nb, "", "c.txt", "kept")
	deletedWithNode := `SELECT count(*) FROM asset_blobs b JOIN nodes n ON n.id = b.node_id
		WHERE n.deleted_at IS NOT NULL AND b.deleted_at = n.deleted_at`

	tm.send(t, nodeDeletion("alice", alone.ID), http.StatusNoContent)
	if n := count(t, tm.pool, deletedWithNode); n != 1 {
		t.Errorf("%d rows deleted with their node, want d.txt's", n)
	}
	tm.send(t, nodeDeletion("alice", root), http.StatusNoContent)
	if n := count(t, tm.pool, deletedWithNode); n != 3 {
		t.Errorf("%d rows deleted with their node, want those of a.png, b.pdf and d.txt", n)
	}
	if status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/assets/"+underChild.ID, tm.tokens["alice"], ""); status != http.StatusNotFound ||
		problemCode(t, answer) != "asset.not_found" {
		t.Errorf("GET a.png = %d %s, want 404 asset.not_found", status, answer)
	}
	if status, _ := tm.download(t, underChild.ContentURL); status != http.StatusNotFound {
		t.Errorf("a.png's address = %d, want 404", status)
	}
	if status, body := tm.download(t, kept.ContentURL); status != http.StatusOK || body != "kept" {
		t.Errorf("c.txt's address = %d %q, want it", status, body)
	}
	checkPages(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}

// An attachment's row follows its notebook (M7/P2 design 3.8) on each of
// the deletion's three paths through serve: the rows go at the notebook's
// time.
func TestDeletingANotebookDeletesItsAttachmentsRows(t *testing.T) {
	for _, tt := range []struct {
		name     string
		setup    func(t *testing.T) (tm acmeTeam, nb, writer string)
		before   func(t *testing.T, tm acmeTeam)
		deletion func(nb string) step
	}{
		{
			name: "by its admin",
			setup: func(t *testing.T) (acmeTeam, string, string) {
				tm := newAcmeTeam(t, "member", "")
				return tm, tm.createNotebook(t, "alice", "Eng"), "alice"
			},
			deletion: func(nb string) step { return request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, "") },
		},
		{
			name: "with its workspace",
			setup: func(t *testing.T) (acmeTeam, string, string) {
				tm := newAcmeTeam(t, "member", "")
				return tm, tm.createNotebook(t, "bob", "Eng"), "bob"
			},
			deletion: func(string) step { return request("alice", http.MethodDelete, "/api/v0/workspaces/acme", "") },
		},
		{
			name: "ownerless, by the workspace's admin",
			setup: func(t *testing.T) (acmeTeam, string, string) {
				tm := newAcmeTeam(t, "admin", "member")
				return tm, tm.createNotebook(t, "carol", "Plans"), "carol"
			},
			before:   func(t *testing.T, tm acmeTeam) { tm.send(t, tm.removal("alice", "carol"), http.StatusNoContent) },
			deletion: func(nb string) step { return ownerlessDeletion("alice", nb) },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm, nb, writer := tt.setup(t)
			page := tm.createPage(t, writer, nb, "", "Root")
			tm.upload(t, writer, nb, page, "a.png", pngFile)
			tm.upload(t, writer, nb, "", "b.txt", "b")
			if tt.before != nil {
				tt.before(t, tm)
			}
			tm.send(t, tt.deletion(nb), http.StatusNoContent)
			if n := count(t, tm.pool, `SELECT count(*) FROM asset_blobs b JOIN notebooks n ON n.id = b.notebook_id
				WHERE n.id = $1 AND n.deleted_at IS NOT NULL AND b.deleted_at = n.deleted_at`, nb); n != 2 {
				t.Errorf("%d rows deleted at the notebook's time, want both", n)
			}
			checkPages(t, tm.pool)
			checkAssets(t, tm.pool, tm.storage)
		})
	}
}

// The attachments' activity reaches the ownerless list through serve (M3
// handoff to M7, item 2): a notebook's size counts its attachments' bytes
// beside its pages', one deleted not, and its last activity is its latest
// upload, after its pages' last write.
func TestTheOwnerlessListShowsTheAttachmentsActivity(t *testing.T) {
	tm := newAcmeTeam(t, "admin", "member")
	nb := tm.createNotebook(t, "carol", "Plans")
	content := "# Notes\n"
	tm.createPageWith(t, "carol", nb, "", "Notes", content)
	gone := tm.upload(t, "carol", nb, "", "gone.bin", strings.Repeat("y", 1000))
	tm.send(t, nodeDeletion("carol", gone.ID), http.StatusNoContent)
	file := strings.Repeat("x", 100)
	uploaded := tm.upload(t, "carol", nb, "", "data.bin", file)
	tm.send(t, tm.removal("alice", "carol"), http.StatusNoContent)

	status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/workspaces/acme/ownerless-notebooks", tm.tokens["alice"], "")
	var list struct {
		Data []struct {
			ID             string    `json:"id"`
			LastActivityAt time.Time `json:"last_activity_at"`
			SizeBytes      int       `json:"size_bytes"`
		} `json:"data"`
	}
	decodeAnswer(t, answer, &list)
	if status != http.StatusOK || len(list.Data) != 1 || list.Data[0].ID != nb {
		t.Fatalf("the ownerless list = %d %s, want Plans alone", status, answer)
	}
	if got := list.Data[0]; got.SizeBytes != len(content)+len(file) || !got.LastActivityAt.Equal(uploaded.CreatedAt) {
		t.Errorf("Plans is listed with %d bytes, last active %s; want %d, at the upload %s", got.SizeBytes, got.LastActivityAt,
			len(content)+len(file), uploaded.CreatedAt)
	}
	checkAssets(t, tm.pool, tm.storage)
}

// An attachment is no link target (M7/P2 design 3.3): a page's [[x.png]]
// stays unresolved with x.png beside it, and renaming the attachment
// writes no page again.
func TestAnAttachmentIsNoLinkTarget(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	page := tm.createPageWith(t, "alice", nb, "", "Notes", "[[x.png]]\n")
	x := tm.upload(t, "alice", nb, "", "x.png", pngFile)
	resolved := "SELECT coalesce(resolved_id::text, 'none') FROM page_links WHERE source_id = $1"
	if got := queryStrings(t, tm.pool, resolved, page); len(got) != 1 || got[0] != "none" {
		t.Errorf("[[x.png]] resolves to %v, want nothing", got)
	}
	tm.send(t, nodeRename("alice", x.ID, "y.png"), http.StatusOK)
	if n := count(t, tm.pool, "SELECT revision FROM page_contents WHERE node_id = $1", page); n != 1 {
		t.Errorf("Notes is at revision %d after the attachment's rename, want 1", n)
	}
	if got := queryStrings(t, tm.pool, resolved, page); len(got) != 1 || got[0] != "none" {
		t.Errorf("[[x.png]] resolves to %v after the rename, want nothing", got)
	}
	checkPages(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}

// The downloads take from their own bucket (M7/P2 design 3.6): with the
// anonymous one empty, an address still downloads, until the downloads'
// own bucket runs out.
func TestDownloadsHaveABucketOfTheirOwn(t *testing.T) {
	tm := newAcmeTeamWith(t, "member", "", func(c *config.Config) {
		c.RateLimit.Anonymous = config.BucketConfig{PerMinute: 1, Burst: 6}
		c.RateLimit.AssetContent = config.BucketConfig{PerMinute: 1, Burst: 2}
	})
	nb := tm.openNotebook(t, "alice", "Eng")
	a := tm.upload(t, "alice", nb, "", "a.txt", "abc")
	for status := 0; status != http.StatusTooManyRequests; {
		status, _ = ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/instance", "", "")
	}
	for i := range 2 {
		if status, body := tm.download(t, a.ContentURL); status != http.StatusOK || body != "abc" {
			t.Fatalf("download %d with the anonymous bucket empty = %d %s, want 200", i+1, status, body)
		}
	}
	if status, answer := tm.download(t, a.ContentURL); status != http.StatusTooManyRequests || problemCode(t, answer) != "rate_limited" {
		t.Errorf("a third download = %d %s, want 429 rate_limited", status, answer)
	}
}
