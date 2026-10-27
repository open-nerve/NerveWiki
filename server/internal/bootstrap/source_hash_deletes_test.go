package bootstrap

import (
	"bytes"
	"crypto/sha256"
	"net/http"
	"testing"
	"time"
)

// Both deletion statements clear the derived hashes with the nodes. These
// requests exercise a subtree, an attachment alone and all three notebook
// deletion routes, with nonempty pages and zero-byte attachments already
// carrying their real hashes. Old trash keeps its own deletion time.
func TestSourceDeletesClearEveryHash(t *testing.T) {
	for _, path := range []string{"subtree", "notebook", "workspace", "ownerless"} {
		t.Run(path, func(t *testing.T) {
			tm := newAcmeTeam(t, "admin", "member")
			writer := "alice"
			if path == "ownerless" {
				writer = "carol"
			}
			wiki := tm.createWith(t, tm.tokens[writer], `{"name":"To delete","mode":"llm_wiki"}`)
			if wiki.SourceRootID == nil {
				t.Fatal("created LLM Wiki has no source root")
			}
			raw := *wiki.SourceRootID
			folder := tm.createPage(t, writer, wiki.ID, raw, "Folder")
			page := tm.createPageWith(t, writer, wiki.ID, folder, "Source", "source bytes\r\n")
			asset := tm.upload(t, writer, wiki.ID, page, "empty.bin", "")
			alone := tm.upload(t, writer, wiki.ID, raw, "alone.txt", "alone")
			kept := tm.createPageWith(t, writer, wiki.ID, raw, "Sibling", "kept source")
			old := tm.createPageWith(t, writer, wiki.ID, raw, "Old trash", "previously deleted")

			// A second workspace's source must survive every cascade, including
			// the deletion of acme. Its equal bytes also keep notebook scope real.
			tm.send(t, request("alice", http.MethodPost, "/api/v0/workspaces", `{"name":"Kept","slug":"kept"}`), http.StatusCreated)
			status, body := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces/kept/notebooks",
				tm.tokens["alice"], `{"name":"Other wiki","mode":"llm_wiki"}`)
			var other createdNotebook
			decodeAnswer(t, body, &other)
			if status != http.StatusCreated || other.SourceRootID == nil {
				t.Fatalf("create other wiki = %d %s, want 201 with source root", status, body)
			}
			foreign := tm.createPageWith(t, "alice", other.ID, *other.SourceRootID, "Source", "source bytes\r\n")
			checkLiveHash := func(id, content string) {
				t.Helper()
				var hash []byte
				var live bool
				if err := tm.pool.QueryRow(t.Context(), "SELECT source_hash, deleted_at IS NULL FROM nodes WHERE id = $1", id).
					Scan(&hash, &live); err != nil {
					t.Fatal(err)
				}
				want := sha256.Sum256([]byte(content))
				if !live || !bytes.Equal(hash, want[:]) {
					t.Fatalf("live source %s has hash %x, live=%t; want %x, true", id, hash, live, want)
				}
			}
			for _, source := range [][2]string{{page, "source bytes\r\n"}, {asset.ID, ""}, {alone.ID, "alone"},
				{kept, "kept source"}, {old, "previously deleted"}, {foreign, "source bytes\r\n"}} {
				checkLiveHash(source[0], source[1])
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)

			deletedAt := func(id string) time.Time {
				t.Helper()
				var at *time.Time
				var hash []byte
				if err := tm.pool.QueryRow(t.Context(), "SELECT deleted_at, source_hash FROM nodes WHERE id = $1", id).
					Scan(&at, &hash); err != nil {
					t.Fatal(err)
				}
				if at == nil || hash != nil {
					t.Fatalf("deleted node %s has time %v and hash %x, want a deletion time and SQL NULL", id, at, hash)
				}
				return *at
			}
			tm.send(t, nodeDeletion(writer, old), http.StatusNoContent)
			oldAt := deletedAt(old)
			tm.send(t, nodeDeletion(writer, alone.ID), http.StatusNoContent)
			aloneAt := deletedAt(alone.ID)
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)

			switch path {
			case "subtree":
				tm.send(t, nodeDeletion(writer, folder), http.StatusNoContent)
				checkLiveHash(kept, "kept source")
			case "notebook":
				tm.send(t, request(writer, http.MethodDelete, "/api/v0/notebooks/"+wiki.ID, ""), http.StatusNoContent)
			case "workspace":
				tm.send(t, request("alice", http.MethodDelete, "/api/v0/workspaces/acme", ""), http.StatusNoContent)
			case "ownerless":
				tm.send(t, tm.removal("alice", writer), http.StatusNoContent)
				tm.send(t, ownerlessDeletion("alice", wiki.ID), http.StatusNoContent)
			}
			at := deletedAt(folder)
			for _, id := range []string{page, asset.ID} {
				if got := deletedAt(id); !got.Equal(at) {
					t.Errorf("descendant %s deleted at %s, want the folder's %s", id, got, at)
				}
			}
			if got := deletedAt(old); !got.Equal(oldAt) {
				t.Errorf("old trash deleted at %s, want its original %s", got, oldAt)
			}
			if got := deletedAt(alone.ID); !got.Equal(aloneAt) {
				t.Errorf("attachment's earlier deletion changed from %s to %s", aloneAt, got)
			}
			if path != "subtree" {
				var notebookAt *time.Time
				if err := tm.pool.QueryRow(t.Context(), "SELECT deleted_at FROM notebooks WHERE id = $1", wiki.ID).Scan(&notebookAt); err != nil {
					t.Fatal(err)
				}
				if notebookAt == nil || !notebookAt.Equal(at) {
					t.Errorf("notebook deleted at %v, want its live nodes' %s", notebookAt, at)
				}
				if n := count(t, tm.pool, `SELECT count(*) FROM nodes WHERE notebook_id = $1
					AND (deleted_at IS NULL OR source_hash IS NOT NULL)`, wiki.ID); n != 0 {
					t.Errorf("deleted notebook keeps %d live or hashed nodes, want none", n)
				}
			}
			checkLiveHash(foreign, "source bytes\r\n")
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkPages(t, tm.pool)
			checkAssets(t, tm.pool, tm.storage)
		})
	}
}
