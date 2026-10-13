package bootstrap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The history through serve (M8/P3 design 3.12): a notebook's log read,
// filtered and paged, a changeset's changes, and each kind of change
// reverted whole through the API, with the conflicts and the refusals of
// a revert. The page module's history reaches the API through the
// composition root: a part left out fails these tests.

// changesetAnswer is a Changeset answer, or a ChangesetDetail's, as much
// as the tests read.
type changesetAnswer struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Client    string  `json:"client"`
	Message   *string `json:"message"`
	CreatedBy struct {
		UserID      string `json:"user_id"`
		DisplayName string `json:"display_name"`
	} `json:"created_by"`
	Reverts *struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	} `json:"reverts"`
	RevertedBy   []string       `json:"reverted_by"`
	Trimmed      bool           `json:"trimmed"`
	NodeCount    int            `json:"node_count"`
	ContentCount int            `json:"content_count"`
	Changes      []changeAnswer `json:"changes"`
	Renames      bool           `json:"renames"`
	Moves        bool           `json:"moves"`
}

// changeAnswer is a ChangesetChange answer.
type changeAnswer struct {
	Node struct {
		ID    string `json:"id"`
		Kind  string `json:"kind"`
		Name  string `json:"name"`
		Alive bool   `json:"alive"`
	} `json:"node"`
	Kind    string       `json:"kind"`
	Before  *changeState `json:"before"`
	After   *changeState `json:"after"`
	Content *struct {
		BaseRevision *int `json:"base_revision"`
		Revision     int  `json:"revision"`
	} `json:"content"`
}

// changeState is a ChangeState answer.
type changeState struct {
	ParentID   *string `json:"parent_id"`
	ParentName *string `json:"parent_name"`
	Name       string  `json:"name"`
}

// String is the state as the tests compare it: its parent, its parent's
// name and its name.
func (s *changeState) String() string {
	if s == nil {
		return "none"
	}
	parent, name := "", ""
	if s.ParentID != nil {
		parent = *s.ParentID
	}
	if s.ParentName != nil {
		name = *s.ParentName
	}
	return parent + "/" + name + "/" + s.Name
}

// history reads a page of the notebook nb's log as by, with the query
// parameters query: its changesets, and its next cursor, "" at its end.
func (tm acmeTeam) history(t *testing.T, by, nb, query string) ([]changesetAnswer, string) {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+nb+"/changesets?"+query, tm.tokens[by], "")
	if status != http.StatusOK {
		t.Fatalf("GET the log of %s?%s as %s = %d %s", nb, query, by, status, body)
	}
	var l struct {
		Data       []changesetAnswer `json:"data"`
		NextCursor *string           `json:"next_cursor"`
	}
	decodeAnswer(t, body, &l)
	if l.NextCursor == nil {
		return l.Data, ""
	}
	return l.Data, *l.NextCursor
}

// newest is the newest changeset of the notebook nb, as alice reads its
// log.
func (tm acmeTeam) newest(t *testing.T, nb string) changesetAnswer {
	t.Helper()
	l, _ := tm.history(t, "alice", nb, "limit=1")
	if len(l) == 0 {
		t.Fatalf("the log of %s is empty", nb)
	}
	return l[0]
}

// changeset reads the changeset id as by.
func (tm acmeTeam) changeset(t *testing.T, by, id string) changesetAnswer {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/changesets/"+id, tm.tokens[by], "")
	if status != http.StatusOK {
		t.Fatalf("GET the changeset %s as %s = %d %s", id, by, status, body)
	}
	var c changesetAnswer
	decodeAnswer(t, body, &c)
	return c
}

// cursorParam is the query parameter of cursor, none when it is "".
func cursorParam(cursor string) string {
	if cursor == "" {
		return ""
	}
	return "&cursor=" + url.QueryEscape(cursor)
}

// changesetRevert is by's revert of the changeset id, without a message.
func changesetRevert(by, id string) step {
	return request(by, http.MethodPost, "/api/v0/changesets/"+id+"/revert", "")
}

// revertAnswer is a ChangesetReverted answer.
type revertAnswer struct {
	ChangesetID *string `json:"changeset_id"`
	Changed     bool    `json:"changed"`
}

// reverted reverts the changeset id as by, which must change something,
// checks the invariants, and answers the revert's changeset.
func (tm acmeTeam) reverted(t *testing.T, by, id string) string {
	t.Helper()
	c := changesetRevert(by, id)
	status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens[by], c.body)
	var r revertAnswer
	decodeAnswer(t, body, &r)
	if status != http.StatusOK || !r.Changed || r.ChangesetID == nil {
		t.Fatalf("revert %s as %s = %d %s, want 200, changed", id, by, status, body)
	}
	tm.checkTrash(t)
	return *r.ChangesetID
}

// revertConflict is a RevertConflict answer.
type revertConflict struct {
	NodeID   string  `json:"node_id"`
	Name     string  `json:"name"`
	Reason   string  `json:"reason"`
	HolderID *string `json:"holder_id"`
}

// conflictsOf is the conflicts a's problem names, and their count.
func conflictsOf(t *testing.T, a answer) ([]revertConflict, int) {
	t.Helper()
	var p struct {
		Conflicts     []revertConflict `json:"conflicts"`
		ConflictCount int              `json:"conflict_count"`
	}
	if err := json.Unmarshal([]byte(a.body), &p); err != nil {
		t.Fatalf("the problem %s: %v", a.body, err)
	}
	return p.Conflicts, p.ConflictCount
}

// notebookState is the notebook nb as a revert puts it back (M8/P3
// design 3.12): its nodes alive, each with its parent, its name, its
// place among its siblings and its content's bytes, but for the pages
// except, whose writes are the test's; their links in the index, each
// where it resolves; the attachments' rows alive. Revisions and times are
// left out: a revert writes its own.
func (tm acmeTeam) notebookState(t *testing.T, nb string, except ...string) string {
	t.Helper()
	var s string
	if err := tm.pool.QueryRow(t.Context(), `SELECT coalesce(string_agg(x, E'\n' ORDER BY x), '') FROM (
			SELECT concat_ws(' ', 'node', n.id, n.parent_id, n.name, rank() OVER (PARTITION BY n.parent_id ORDER BY n.sort_order, n.id),
				CASE WHEN n.id::text = ANY($2) THEN NULL ELSE md5(c.content) END) AS x
			FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL
			WHERE n.notebook_id = $1 AND n.deleted_at IS NULL
			UNION ALL
			SELECT concat_ws(' ', 'link', l.source_id, l.range_start, l.target, l.resolved_id) FROM page_links l WHERE l.notebook_id = $1
			UNION ALL
			SELECT concat_ws(' ', 'blob', b.node_id, b.id) FROM asset_blobs b WHERE b.notebook_id = $1 AND b.deleted_at IS NULL
		) s`, nb, except).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// undone makes change in the notebook nb, its newest changeset then of
// kind, which bob reverts, between running before it when it is not nil:
// the revert is bob's, of the changeset, and the notebook is as it was
// before the change, but for the contents of the pages except. It
// answers the changeset reverted.
func (tm acmeTeam) undone(t *testing.T, nb, kind string, change, between func(), except ...string) changesetAnswer {
	t.Helper()
	was := tm.notebookState(t, nb, except...)
	change()
	c := tm.newest(t, nb)
	if c.Kind != kind {
		t.Fatalf("the change's changeset is of %s, want %s", c.Kind, kind)
	}
	if between != nil {
		between()
	}
	r := tm.reverted(t, "bob", c.ID)
	got := tm.newest(t, nb)
	if got.ID != r || got.Kind != "revert" || got.Reverts == nil || got.Reverts.ID != c.ID || got.Reverts.Kind != kind ||
		got.CreatedBy.UserID != tm.userID(t, "bob").String() {
		t.Errorf("the log's newest = %+v, want bob's revert %s of %s", got, r, c.ID)
	}
	if now := tm.notebookState(t, nb, except...); now != was {
		t.Errorf("the notebook after the revert:\n%s\nwant as it was:\n%s", now, was)
	}
	return c
}

// Each kind of change made through the API is reverted whole by bob, an
// editor who did not make it, in a revert changeset of his: the notebook
// is as it was before, its pages' bytes, its tree with each node's place
// among its siblings, its link index and its attachments' rows. A content
// an edit session wrote; a rename, whose rewrite of Linker's link is
// written back byte for byte; a move; a reorder; a page's creation and an
// attachment's, whose row goes with it; a subtree's deletion with its
// attachment, whose row comes back and downloads at a new address, the
// streams seeing the tree and the links as a restore's; a restore from the
// trash; a version's restore; an import; and a revert's revert, which
// makes the change again, with its message, each telling the log what
// reverted it.
func TestEachChangeIsRevertedThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	marker := tm.createPage(t, "alice", nb, "", "Marker")
	target := tm.createPage(t, "alice", nb, "", "Target")
	linker := tm.createPageWith(t, "alice", nb, "", "Linker", "see [[Target]]\n")
	p := tm.createPage(t, "alice", nb, "", "P")
	x := tm.createPage(t, "alice", nb, "", "X")

	t.Run("an edit session's content", func(t *testing.T) {
		tm.undone(t, nb, "edit", func() {
			session := tm.openSession(t, "alice", linker)
			rev := tm.content(t, "alice", linker).Revision
			tm.send(t, contentWrite("alice", linker, "first", rev, session), http.StatusOK)
			tm.send(t, contentWrite("alice", linker, "second", rev+1, session), http.StatusOK)
			tm.send(t, request("alice", http.MethodDelete, "/api/v0/edit-sessions/"+session, ""), http.StatusNoContent)
		}, nil)
	})
	t.Run("a rename that wrote a link again", func(t *testing.T) {
		var s *eventStream
		tm.undone(t, nb, "edit", func() {
			tm.send(t, nodeRename("alice", target, "Goal"), http.StatusOK)
			if c := tm.content(t, "alice", linker); c.Content != "see [[Goal]]\n" {
				t.Fatalf("Linker after the rename = %q, want its link written again", c.Content)
			}
		}, func() {
			tm.settle(t, nb, marker)
			s = openStream(t, tm.base, "bob", tm.tokens["bob"])
		}, marker)
		c := tm.content(t, "alice", linker)
		if c.Content != "see [[Target]]\n" {
			t.Errorf("Linker after the revert = %q, want its bytes before the rename", c.Content)
		}
		s.tree(t, nb, map[string]int{linker: c.Revision})
		s.links(t, nb, []string{}, []string{target})
		tm.quiet(t, nb, marker, s)
		tm.resolves(t, linker, target)
	})
	t.Run("a move", func(t *testing.T) {
		tm.undone(t, nb, "edit", func() { tm.send(t, nodeMove("alice", x, p), http.StatusOK) }, nil)
	})
	t.Run("a reorder", func(t *testing.T) {
		tm.undone(t, nb, "edit", func() {
			tm.send(t, request("alice", http.MethodPost, "/api/v0/nodes/"+x+"/move", `{"parent_id":null,"after_id":null}`), http.StatusOK)
		}, nil)
	})
	t.Run("a page's creation", func(t *testing.T) {
		var fresh string
		tm.undone(t, nb, "edit", func() { fresh = tm.createPageWith(t, "alice", nb, p, "Fresh", "[[Target]]") }, nil)
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NOT NULL AND trash_root", fresh); n != 1 {
			t.Errorf("Fresh is not a trash item of its own")
		}
	})
	t.Run("an attachment's upload", func(t *testing.T) {
		var up uploadedAsset
		tm.undone(t, nb, "edit", func() { up = tm.upload(t, "alice", nb, p, "up.png", sevenByFive) }, nil)
		if n := count(t, tm.pool, "SELECT count(*) FROM asset_blobs b JOIN nodes n ON n.id = b.node_id WHERE n.id = $1 "+
			"AND b.deleted_at = n.deleted_at", up.ID); n != 1 {
			t.Errorf("up.png's row is not deleted with its node")
		}
	})
	t.Run("a subtree's deletion", func(t *testing.T) {
		r := tm.createPage(t, "alice", nb, "", "R")
		a := tm.createPageWith(t, "alice", nb, r, "A", "# A")
		b := tm.createPage(t, "alice", nb, r, "B")
		photo := tm.upload(t, "alice", nb, a, "photo.png", sevenByFive)
		tm.send(t, contentWrite("alice", a, "[[B]] ![[photo.png]]", 1, ""), http.StatusOK)
		pointer := tm.createPageWith(t, "alice", nb, "", "Pointer", "[[A]]")
		var s *eventStream
		tm.undone(t, nb, "edit", func() { tm.send(t, nodeDeletion("alice", r), http.StatusNoContent) }, func() {
			tm.settle(t, nb, marker)
			s = openStream(t, tm.base, "bob", tm.tokens["bob"])
		}, marker)
		s.tree(t, nb, map[string]int{r: 1, a: 2, b: 1})
		s.links(t, nb, []string{pointer}, []string{a, b, photo.ID})
		tm.quiet(t, nb, marker, s)
		status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/assets/"+photo.ID, tm.tokens["alice"], "")
		if status != http.StatusOK {
			t.Fatalf("GET photo.png = %d %s, want 200", status, body)
		}
		if status, file := tm.download(t, tm.asset(t, body).ContentURL); status != http.StatusOK || file != sevenByFive {
			t.Errorf("photo.png's new address = %d, want its file", status)
		}
		if v := tm.versions(t, "alice", a); len(v) != 2 {
			t.Errorf("A's versions = %+v, want both back", v)
		}
	})
	t.Run("a restore from the trash", func(t *testing.T) {
		gone := tm.createPage(t, "alice", nb, p, "Gone")
		tm.send(t, nodeDeletion("alice", gone), http.StatusNoContent)
		tm.undone(t, nb, "trash_restore", func() { tm.restored(t, "alice", gone) }, nil)
		if items := tm.trash(t, "alice", nb); len(items) == 0 || items[0].ID != gone {
			t.Errorf("the trash = %+v, want Gone first, deleted again", items)
		}
	})
	t.Run("a version's restore", func(t *testing.T) {
		v := tm.createPageWith(t, "alice", nb, "", "V", "one")
		tm.send(t, contentWrite("alice", v, "two", 1, ""), http.StatusOK)
		tm.undone(t, nb, "version_restore", func() { tm.send(t, versionRestore("alice", v, 1, 2), http.StatusOK) }, nil)
		if c := tm.content(t, "alice", v); c.Content != "two" || c.Revision != 4 {
			t.Errorf("V = %+v, want two again, at revision 4", c)
		}
	})
	t.Run("an import", func(t *testing.T) {
		tm.undone(t, nb, "import", func() {
			j := tm.imported(t, "alice", nb, "", vaultZip(t, vaultFile{"Imported.md", "[[Target]] ![[pic.png]]"},
				vaultFile{"Imported/pic.png", pngHead}, vaultFile{"Imported/Under.md", "under"}))
			if j.State != "succeeded" {
				t.Fatalf("the import = %+v, want succeeded", j)
			}
		}, nil)
	})
	t.Run("a revert's revert", func(t *testing.T) {
		tm.send(t, nodeRename("alice", x, "Y"), http.StatusOK)
		renamed := tm.newest(t, nb)
		was := tm.notebookState(t, nb)
		first := tm.reverted(t, "bob", renamed.ID)
		c := changesetRevert("alice", first)
		c.body = `{"message":"Y after all"}`
		status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["alice"], c.body)
		if status != http.StatusOK {
			t.Fatalf("the revert's revert = %d %s, want 200", status, body)
		}
		var second revertAnswer
		decodeAnswer(t, body, &second)
		if now := tm.notebookState(t, nb); now != was {
			t.Errorf("the notebook after the revert's revert:\n%s\nwant as the rename left it:\n%s", now, was)
		}
		l, _ := tm.history(t, "alice", nb, "limit=3")
		if len(l) != 3 || second.ChangesetID == nil || l[0].ID != *second.ChangesetID || l[0].Message == nil || *l[0].Message != "Y after all" ||
			l[0].Reverts == nil || l[0].Reverts.ID != first || l[0].Reverts.Kind != "revert" ||
			fmt.Sprint(l[1].RevertedBy) != fmt.Sprint([]string{*second.ChangesetID}) ||
			fmt.Sprint(l[2].RevertedBy) != fmt.Sprint([]string{first}) {
			t.Errorf("the log = %+v; want the second revert, with its message, of the first, of the rename, each reverted by the next", l)
		}
		tm.checkTrash(t)
	})
}

// The log through serve (M8/P3 design 3.4): a notebook's changesets newer
// first, each with who made it and from where, its counts and its first
// changes, where each node was before and after it; filtered by its
// actor, its client, or a node it changed, one in the trash among them;
// a page at a time. A changeset read alone tells whether it renames or
// moves, and its changes come a page at a time. A revert tells what it
// reverts, with its message, and the changeset what reverted it.
func TestTheLogThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	tm.tokens["alice's token"] = createToken(t, tm.contract, tm.base, tm.tokens["alice"])
	nb := tm.openNotebook(t, "alice", "Eng")
	elsewhere := tm.createPage(t, "alice", tm.openNotebook(t, "alice", "Other"), "", "Elsewhere")
	a := tm.createPage(t, "alice", nb, "", "A")
	tm.send(t, nodeRename("bob", a, "B"), http.StatusOK)
	tm.send(t, pageCreation("alice's token", nb, "Tokened"), http.StatusCreated)
	tm.send(t, contentWrite("alice", a, "# B", 1, ""), http.StatusOK)
	box := tm.createPage(t, "alice", nb, "", "Box")
	tm.send(t, nodeMove("alice", a, box), http.StatusOK)
	child := tm.createPage(t, "alice", nb, a, "Child")
	tm.send(t, nodeDeletion("alice", a), http.StatusNoContent)

	all, next := tm.history(t, "bob", nb, "")
	if len(all) != 8 || next != "" {
		t.Fatalf("the log = %d changesets, next %q; want 8, and its end", len(all), next)
	}
	byName := map[string]changesetAnswer{"deletion": all[0], "child": all[1], "move": all[2], "box": all[3], "write": all[4],
		"tokened": all[5], "rename": all[6], "creation": all[7]}
	alice, bob := tm.userID(t, "alice").String(), tm.userID(t, "bob").String()
	for name, want := range map[string]string{
		"deletion": "web alice 2 0 deleted:" + box + "/Box/B>none deleted:" + a + "/B/Child>none",
		"child":    "web alice 1 1 created:none>" + a + "/B/Child@0>1",
		"move":     "web alice 1 0 moved:/B>" + box + "/Box/B",
		"box":      "web alice 1 1 created:none>/Box@0>1",
		"write":    "web alice 0 1 content:none>none@1>2",
		"tokened":  "api alice 1 1 created:none>/Tokened@0>1",
		"rename":   "web bob 1 0 renamed:/A>/B",
		"creation": "web alice 1 1 created:none>/A@0>1",
	} {
		c := byName[name]
		by := map[string]string{alice: "alice", bob: "bob"}[c.CreatedBy.UserID]
		got := fmt.Sprintf("%s %s %d %d", c.Client, by, c.NodeCount, c.ContentCount)
		for _, ch := range c.Changes {
			got += " " + ch.Kind + ":" + ch.Before.String() + ">" + ch.After.String()
			if ch.Content != nil {
				base := 0
				if ch.Content.BaseRevision != nil {
					base = *ch.Content.BaseRevision
				}
				got += fmt.Sprintf("@%d>%d", base, ch.Content.Revision)
			}
		}
		got = strings.ReplaceAll(got, "//", "/")
		if want = strings.ReplaceAll(want, "//", "/"); got != want || c.Kind != "edit" || c.Trimmed || c.Message != nil ||
			len(c.RevertedBy) != 0 || c.Reverts != nil || c.CreatedBy.DisplayName != tm.displayName(t, by) {
			t.Errorf("the %s = %+v, as %q; want %q", name, c, got, want)
		}
	}

	for query, want := range map[string][]changesetAnswer{
		"actor=" + bob:                   {byName["rename"]},
		"client=api":                     {byName["tokened"]},
		"client=web":                     {all[0], all[1], all[2], all[3], all[4], all[6], all[7]},
		"client=mcp":                     {},
		"client=mcp:claude":              {},
		"page_id=" + a:                   {byName["deletion"], byName["move"], byName["write"], byName["rename"], byName["creation"]},
		"page_id=" + child:               {byName["deletion"], byName["child"]},
		"actor=" + alice + "&client=api": {byName["tokened"]},
	} {
		got, _ := tm.history(t, "bob", nb, query)
		if len(got) != len(want) || (len(want) > 0 && !reflect.DeepEqual(got, want)) {
			t.Errorf("the log of %s = %d changesets, want %d", query, len(got), len(want))
		}
	}
	for query, code := range map[string]string{"client=bad": "validation_failed", "page_id=" + elsewhere: "page.not_found",
		"limit=0": "validation_failed", "cursor=x": "bad_request"} {
		status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+nb+"/changesets?"+query, tm.tokens["bob"], "")
		if status/100 != 4 || problemCode(t, body) != code {
			t.Errorf("the log of %s = %d %s, want %s", query, status, body, code)
		}
	}
	var paged []changesetAnswer
	for cursor, pages := "", 0; pages == 0 || cursor != ""; pages++ {
		l, next := tm.history(t, "bob", nb, "limit=3"+cursorParam(cursor))
		paged, cursor = append(paged, l...), next
		if pages > 3 {
			t.Fatal("the log does not end")
		}
	}
	if !reflect.DeepEqual(paged, all) {
		t.Errorf("the log 3 at a time = %+v, want %+v", paged, all)
	}

	if c := tm.changeset(t, "bob", byName["move"].ID); !c.Moves || c.Renames || c.ID != byName["move"].ID {
		t.Errorf("the move read alone = %+v, want it moving", c)
	}
	if c := tm.changeset(t, "bob", byName["rename"].ID); c.Moves || !c.Renames {
		t.Errorf("the rename read alone = %+v, want it renaming", c)
	}
	var changes []changeAnswer
	for cursor, pages := "", 0; pages == 0 || cursor != ""; pages++ {
		status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/changesets/"+byName["deletion"].ID+"/changes?limit=1"+cursorParam(cursor),
			tm.tokens["bob"], "")
		var l struct {
			Data       []changeAnswer `json:"data"`
			NextCursor *string        `json:"next_cursor"`
		}
		decodeAnswer(t, body, &l)
		if status != http.StatusOK || len(l.Data) != 1 || pages > 2 {
			t.Fatalf("the deletion's changes after %q = %d %s, want one", cursor, status, body)
		}
		changes, cursor = append(changes, l.Data...), ""
		if l.NextCursor != nil {
			cursor = *l.NextCursor
		}
	}
	if !reflect.DeepEqual(changes, byName["deletion"].Changes) || changes[0].Node.Alive {
		t.Errorf("the deletion's changes one at a time = %+v, want its first two, B in the trash", changes)
	}

	c := changesetRevert("bob", byName["deletion"].ID)
	c.body = `{"message":"Back"}`
	tm.send(t, c, http.StatusOK)
	l, _ := tm.history(t, "bob", nb, "limit=1")
	if len(l) != 1 || l[0].Kind != "revert" || l[0].Message == nil || *l[0].Message != "Back" || l[0].Reverts == nil ||
		l[0].Reverts.ID != byName["deletion"].ID || l[0].Reverts.Kind != "edit" || l[0].CreatedBy.UserID != bob {
		t.Errorf("the log's newest = %+v, want bob's revert of the deletion, Back", l)
	}
	if d := tm.changeset(t, "bob", byName["deletion"].ID); fmt.Sprint(d.RevertedBy) != fmt.Sprint([]string{l[0].ID}) ||
		!d.Changes[0].Node.Alive {
		t.Errorf("the deletion = %+v, want it reverted by the revert, B alive", d)
	}
	tm.checkTrash(t)
}

// A revert that would not put its changeset's nodes back as they were is
// page.revert_conflict, its conflict named, and writes nothing: a page
// whose content was written since (changed); a deletion past its period
// (expired); a node whose parent before is deleted since (parent_gone); a
// page put under one the revert deletes (children_added); a name another
// took where one goes back (title_taken, naming its holder); a node that
// would go under its own subtree (cycle); a subtree that would go deeper
// than ten levels of pages (too_deep).
func TestARevertsConflictsThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	for _, tt := range []struct {
		reason string
		// make makes the changeset, then what conflicts with its revert:
		// the changeset's id, the conflict's node, name and holder.
		make func(t *testing.T, nb string) (changeset, node, name, holder string)
	}{
		{"changed", func(t *testing.T, nb string) (string, string, string, string) {
			p := tm.createPageWith(t, "alice", nb, "", "P", "one")
			tm.send(t, contentWrite("alice", p, "two", 1, ""), http.StatusOK)
			c := tm.newest(t, nb).ID
			tm.send(t, contentWrite("alice", p, "three", 2, ""), http.StatusOK)
			return c, p, "P", ""
		}},
		{"expired", func(t *testing.T, nb string) (string, string, string, string) {
			d := tm.createPage(t, "alice", nb, "", "D")
			tm.send(t, nodeDeletion("alice", d), http.StatusNoContent)
			tm.trashedAgo(t, d, 1439*time.Hour+time.Minute)
			return tm.newest(t, nb).ID, d, "D", ""
		}},
		{"parent_gone", func(t *testing.T, nb string) (string, string, string, string) {
			p := tm.createPage(t, "alice", nb, "", "P")
			x := tm.createPage(t, "alice", nb, p, "X")
			tm.send(t, nodeMove("alice", x, ""), http.StatusOK)
			c := tm.newest(t, nb).ID
			tm.send(t, nodeDeletion("alice", p), http.StatusNoContent)
			return c, x, "X", ""
		}},
		{"children_added", func(t *testing.T, nb string) (string, string, string, string) {
			z := tm.createPage(t, "alice", nb, "", "Z")
			c := tm.newest(t, nb).ID
			return c, tm.createPage(t, "alice", nb, z, "W"), "W", ""
		}},
		{"title_taken", func(t *testing.T, nb string) (string, string, string, string) {
			x := tm.createPage(t, "alice", nb, "", "A")
			tm.send(t, nodeRename("alice", x, "B"), http.StatusOK)
			c := tm.newest(t, nb).ID
			return c, x, "A", tm.createPage(t, "alice", nb, "", "a")
		}},
		{"cycle", func(t *testing.T, nb string) (string, string, string, string) {
			p := tm.createPage(t, "alice", nb, "", "P")
			x := tm.createPage(t, "alice", nb, p, "X")
			tm.send(t, nodeMove("alice", x, ""), http.StatusOK)
			c := tm.newest(t, nb).ID
			tm.send(t, nodeMove("alice", p, x), http.StatusOK)
			return c, x, "X", ""
		}},
		{"too_deep", func(t *testing.T, nb string) (string, string, string, string) {
			var eighth, levelA string
			for i := range domain.MaxDepth - 2 {
				eighth = tm.createPage(t, "alice", nb, eighth, "Level "+string(rune('A'+i)))
				if i == 0 {
					levelA = eighth
				}
			}
			x := tm.createPage(t, "alice", nb, eighth, "X")
			tm.createPage(t, "alice", nb, x, "Y")
			tm.send(t, nodeMove("alice", x, ""), http.StatusOK)
			c := tm.newest(t, nb).ID
			tm.send(t, nodeMove("alice", levelA, tm.createPage(t, "alice", nb, "", "Top")), http.StatusOK)
			return c, x, "X", ""
		}},
	} {
		t.Run(tt.reason, func(t *testing.T) {
			nb := tm.openNotebook(t, "alice", "Notebook "+tt.reason)
			id, node, name, holder := tt.make(t, nb)
			was := tm.notebookState(t, nb)
			c := changesetRevert("bob", id)
			status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["bob"], "")
			conflicts, n := conflictsOf(t, answer{body: body})
			want := revertConflict{NodeID: node, Name: name, Reason: tt.reason}
			if holder != "" {
				want.HolderID = &holder
			}
			if status != http.StatusConflict || problemCode(t, body) != "page.revert_conflict" || n != 1 || len(conflicts) != 1 ||
				!reflect.DeepEqual(conflicts[0], want) {
				t.Errorf("the revert = %d %s, want 409 page.revert_conflict of %+v", status, body, want)
			}
			if now := tm.notebookState(t, nb); now != was {
				t.Errorf("the refused revert changed the notebook:\n%s\nwas\n%s", now, was)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1 AND kind = 'revert'", nb); n != 0 {
				t.Errorf("%d reverts' changesets, want none", n)
			}
			tm.checkTrash(t)
		})
	}
}

// A changeset part of which the purge took is page.changeset_trimmed, and
// the log tells it trimmed: a rename whose page went past its period in
// the trash, its rewrite of a link alive. A revert past the unit's bounds
// is page.revert_too_large: more than 100 pages, more than 8 MiB of their
// contents, more than 20,000 links among them, known once they are
// parsed; a lone page is never past them. A message past its rules is
// validation_failed, a changeset that is none page.changeset_not_found.
// None writes anything.
func TestARevertTheServerWillNotMakeIsRefused(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	trimmed := tm.createPage(t, "alice", nb, "", "Trimmed")
	tm.createPageWith(t, "alice", nb, "", "Linker", "[[Trimmed]]")
	tm.send(t, nodeRename("alice", trimmed, "Renamed"), http.StatusOK)
	rename := tm.newest(t, nb).ID
	tm.send(t, nodeDeletion("alice", trimmed), http.StatusNoContent)
	tm.trashedAgo(t, trimmed, 1441*time.Hour)
	if err := tm.purgeBefore(t, time.Now().Add(-1440*time.Hour))(); err != nil {
		t.Fatalf("the purge: %v", err)
	}
	if c := tm.changeset(t, "alice", rename); !c.Trimmed || c.ContentCount != 1 {
		t.Errorf("the rename after the purge = %+v, want it trimmed, its version of Linker kept", c)
	}

	many := tm.createPage(t, "alice", nb, "", "Many")
	for i := range domain.MaxUnitPages {
		tm.send(t, childCreation("alice", nb, many, fmt.Sprintf("Page %d", i)), http.StatusCreated)
	}
	heavy := tm.createPageWith(t, "alice", nb, "", "Heavy", strings.Repeat("a", domain.MaxUnitBytes/2+1))
	tm.createPageWith(t, "alice", nb, heavy, "Heavier", strings.Repeat("b", domain.MaxUnitBytes/2))
	links := strings.Repeat("[[x]]\n", domain.MaxUnitLinks/2+1)
	linked := tm.createPageWith(t, "alice", nb, "", "Linked", links)
	tm.createPageWith(t, "alice", nb, linked, "More", links)
	lone := tm.createPageWith(t, "alice", nb, "", "Lone", links+links)
	deletions := map[string]string{}
	for name, id := range map[string]string{"101 pages": many, "8 MiB and a byte": heavy, "20,002 links": linked, "lone": lone} {
		tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)
		deletions[name] = tm.newest(t, nb).ID
	}
	was := tm.notebookState(t, nb)
	for _, tt := range []struct {
		name, id, body string
		status         int
		code           string
	}{
		{"the trimmed rename", rename, "", http.StatusConflict, "page.changeset_trimmed"},
		{"101 pages", deletions["101 pages"], "", http.StatusUnprocessableEntity, "page.revert_too_large"},
		{"8 MiB and a byte", deletions["8 MiB and a byte"], "", http.StatusUnprocessableEntity, "page.revert_too_large"},
		{"20,002 links", deletions["20,002 links"], "", http.StatusUnprocessableEntity, "page.revert_too_large"},
		{"an empty message", deletions["lone"], `{"message":""}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"a message of 4,097 bytes", deletions["lone"], `{"message":"` + strings.Repeat("m", 4097) + `"}`, http.StatusUnprocessableEntity,
			"validation_failed"},
		{"no changeset", uuid.NewV7().String(), "", http.StatusNotFound, "page.changeset_not_found"},
	} {
		c := changesetRevert("bob", tt.id)
		if status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["bob"], tt.body); status != tt.status ||
			problemCode(t, body) != tt.code {
			t.Errorf("revert %s = %d %s, want %d %s", tt.name, status, body, tt.status, tt.code)
		}
	}
	if now := tm.notebookState(t, nb); now != was {
		t.Errorf("the refused reverts changed the notebook")
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE kind = 'revert'"); n != 0 {
		t.Errorf("%d reverts' changesets, want none", n)
	}
	tm.reverted(t, "bob", deletions["lone"])
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL", lone); n != 1 {
		t.Errorf("Lone is not back")
	}
}
