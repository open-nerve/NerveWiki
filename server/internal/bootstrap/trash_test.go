package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The trash through serve (M8/P2 design 3.12): a deletion's item listed and
// read, a page of it viewed from where it was, and restored with all that
// was deleted with it.

// trashItemFull is a TrashItem answer, the whole of it.
type trashItemFull struct {
	ID         string `json:"id"`
	NotebookID string `json:"notebook_id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Ancestors  []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"ancestors"`
	DeletedAt time.Time `json:"deleted_at"`
	DeletedBy struct {
		UserID      string `json:"user_id"`
		DisplayName string `json:"display_name"`
	} `json:"deleted_by"`
	RestorableUntil time.Time `json:"restorable_until"`
	Pages           int       `json:"pages"`
	Assets          int       `json:"assets"`
	RestoreTo       placed    `json:"restore_to"`
	Revision        *int      `json:"revision"`
	Asset           *struct {
		MIME     string `json:"mime"`
		ByteSize int    `json:"byte_size"`
	} `json:"asset"`
}

// placed is a TrashPlacement.
type placed struct {
	ParentID *string `json:"parent_id"`
	Reason   string  `json:"reason"`
}

// at is where the placement puts an item: its parent, "" for the root,
// and why.
func (p placed) at() string {
	parent := ""
	if p.ParentID != nil {
		parent = *p.ParentID
	}
	return parent + " " + p.Reason
}

// nodeRestored is a NodeRestored answer.
type nodeRestored struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Renamed     bool   `json:"renamed"`
	Placed      placed `json:"placed"`
	ChangesetID string `json:"changeset_id"`
}

// nodeRestore is by's restore of the trash item whose root is id.
func nodeRestore(by, id string) step {
	return request(by, http.MethodPost, "/api/v0/nodes/"+id+"/restore", "")
}

// restored restores the item of id as by, checks the invariants, and
// answers what the restore did.
func (tm acmeTeam) restored(t *testing.T, by, id string) nodeRestored {
	t.Helper()
	c := nodeRestore(by, id)
	status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens[by], "")
	if status != http.StatusOK {
		t.Fatalf("restore %s as %s = %d %s, want 200", id, by, status, body)
	}
	var r nodeRestored
	decodeAnswer(t, body, &r)
	tm.checkTrash(t)
	return r
}

// checkTrash checks the invariants the trash keeps: the pages', with the
// index's, the attachments' and the notebooks'.
func (tm acmeTeam) checkTrash(t *testing.T) {
	t.Helper()
	checkPages(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
	checkNotebooks(t, tm.pool)
}

// trash is the first page of the notebook nb's trash as by reads it.
func (tm acmeTeam) trash(t *testing.T, by, nb string) []trashItemFull {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+nb+"/trash", tm.tokens[by], "")
	if status != http.StatusOK {
		t.Fatalf("GET the trash of %s as %s = %d %s", nb, by, status, body)
	}
	var l struct {
		Data []trashItemFull `json:"data"`
	}
	decodeAnswer(t, body, &l)
	return l.Data
}

// trashItem is the item whose root is id as by reads it: the answer's
// status, the item, and its problem's root_id.
func (tm acmeTeam) trashItem(t *testing.T, by, id string) (int, trashItemFull, string) {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/nodes/"+id+"/trash-item", tm.tokens[by], "")
	var it trashItemFull
	var p struct {
		RootID string `json:"root_id"`
	}
	decodeAnswer(t, body, &it)
	decodeAnswer(t, body, &p)
	return status, it, p.RootID
}

// displayName is name's account's display name.
func (tm acmeTeam) displayName(t *testing.T, name string) string {
	t.Helper()
	names := queryStrings(t, tm.pool, "SELECT display_name FROM users WHERE email = $1", name+"@example.com")
	if len(names) != 1 {
		t.Fatalf("the accounts of %s: %v", name, names)
	}
	return names[0]
}

// versionView is the reading view of the page id's version holding
// revision as alice reads it.
func (tm acmeTeam) versionView(t *testing.T, id string, revision int) string {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodGet, fmt.Sprintf("%s/api/v0/pages/%s/versions/%d/view", tm.base, id, revision),
		tm.tokens["alice"], "")
	if status != http.StatusOK {
		t.Fatalf("the view of %s's version %d = %d %s", id, revision, status, body)
	}
	return body
}

// leadsTo reports whether a view's HTML has a link to the node id.
func leadsTo(view, id string) bool {
	return strings.Contains(view, `data-nw-node=\"`+id+`\"`)
}

// A subtree deleted with its attachment and links is one trash item, R,
// with its counts, where it was, who deleted it, until when it can be
// restored and where it goes back; an attachment deleted alone another,
// with its type and size, the newer first. A page under R is no item: its
// root is named. A page of R's version view resolves its links from R/A,
// against the pages alive: [[B]] leads to Other/B, R/B being in the trash,
// and the attachment R/A embeds is no candidate. Restored, R brings back
// its pages with their contents, versions and items, a trash_restore's
// items each restored, a page's with its revision; the attachment's row,
// which downloads at a new address; the index, [[B]] now R/B's and
// Linker's [[A]] resolving again; and the streams see the tree changed
// with the pages at their revisions, then the links that resolve
// otherwise. The instance tells the trash's retention as the item's
// period.
func TestATrashItemGoesAndComesBack(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	marker := tm.createPage(t, "alice", nb, "", "Marker")
	other := tm.createPage(t, "alice", nb, "", "Other")
	otherB := tm.createPage(t, "alice", nb, other, "B")
	r := tm.createPage(t, "alice", nb, "", "R")
	a := tm.createPageWith(t, "alice", nb, r, "A", "# A")
	b := tm.createPage(t, "alice", nb, r, "B")
	photo := tm.upload(t, "alice", nb, a, "photo.png", sevenByFive)
	tm.send(t, contentWrite("alice", a, "[[B]] ![[photo.png]]", 1, ""), http.StatusOK)
	linker := tm.createPageWith(t, "alice", nb, "", "Linker", "[[A]]")
	loose := tm.upload(t, "alice", nb, "", "loose.png", sevenByFive)
	tm.resolves(t, a, b, photo.ID)
	tm.send(t, nodeDeletion("alice", loose.ID), http.StatusNoContent)
	tm.send(t, nodeDeletion("bob", r), http.StatusNoContent)
	tm.resolves(t, linker, "")
	tm.checkTrash(t)

	items := tm.trash(t, "bob", nb)
	if len(items) != 2 || items[0].ID != r || items[1].ID != loose.ID {
		t.Fatalf("the trash = %+v, want R, then loose.png", items)
	}
	item, image := items[0], items[1]
	if item.NotebookID != nb || item.Kind != "page" || item.Name != "R" || item.Pages != 3 || item.Assets != 1 || len(item.Ancestors) != 0 ||
		item.DeletedBy.UserID != tm.userID(t, "bob").String() || item.DeletedBy.DisplayName != tm.displayName(t, "bob") ||
		item.RestoreTo.at() != " original" || item.Revision == nil || *item.Revision != 1 || item.Asset != nil {
		t.Errorf("R's item = %+v; want 3 pages and an attachment, by bob, back at the root, at revision 1", item)
	}
	var info struct {
		Retention int64 `json:"trash_retention_seconds"`
	}
	tm.get(t, "alice", "/api/v0/instance", &info)
	if period := item.RestorableUntil.Sub(item.DeletedAt); period != 1439*time.Hour || int64(period/time.Second) != info.Retention {
		t.Errorf("R restorable for %s, the instance's retention %d s; want both jobs.purge_retention less an hour", period, info.Retention)
	}
	if image.Kind != "asset" || image.Asset == nil || image.Asset.MIME != "image/png" || image.Asset.ByteSize != len(sevenByFive) ||
		image.Revision != nil || image.Pages != 0 || image.Assets != 1 || image.DeletedBy.DisplayName != tm.displayName(t, "alice") {
		t.Errorf("loose.png's item = %+v; want its type and size, by alice", image)
	}
	if status, got, _ := tm.trashItem(t, "bob", r); status != http.StatusOK || !reflect.DeepEqual(got, item) {
		t.Errorf("R's item = %d %+v, want %+v", status, got, item)
	}
	if status, _, root := tm.trashItem(t, "bob", a); status != http.StatusConflict || root != r {
		t.Errorf("A's item = %d, root %q; want 409 naming R", status, root)
	}
	if view := tm.versionView(t, a, 2); !leadsTo(view, otherB) || leadsTo(view, b) || leadsTo(view, photo.ID) {
		t.Errorf("A's view in the trash = %s, want [[B]] leading to Other/B, the attachment unresolved", view)
	}

	tm.settle(t, nb, marker)
	s := openStream(t, tm.base, "bob", tm.tokens["bob"])
	got := tm.restored(t, "bob", r)
	if got.ID != r || got.Kind != "page" || got.Name != "R" || got.Renamed || got.Placed.at() != " original" || got.ChangesetID == "" {
		t.Errorf("restored %+v, want R back at the root as it was", got)
	}
	s.tree(t, nb, map[string]int{r: 1, a: 2, b: 1})
	s.links(t, nb, []string{linker}, []string{a, b, photo.ID})
	tm.quiet(t, nb, marker, s)
	if c := tm.content(t, "alice", a); c.Content != "[[B]] ![[photo.png]]" || c.Revision != 2 {
		t.Errorf("A's content = %+v, want its second version's", c)
	}
	if v := tm.versions(t, "alice", a); len(v) != 2 || v[0].Revision != 2 {
		t.Errorf("A's versions = %+v, want both", v)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM changeset_items i JOIN changesets s ON s.id = i.changeset_id
		WHERE s.id = $1 AND s.kind = 'trash_restore' AND i.restored AND i.before_name IS NULL AND i.deleted_at IS NULL
			AND i.revision IS NOT DISTINCT FROM CASE i.node_id WHEN $2 THEN 2 WHEN $3 THEN NULL ELSE 1 END`,
		got.ChangesetID, a, photo.ID); n != 4 {
		t.Errorf("%d items of the restore, each restored, a page's at its revision; want 4", n)
	}
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/assets/"+photo.ID, tm.tokens["alice"], "")
	if status != http.StatusOK {
		t.Fatalf("GET photo.png = %d %s, want 200", status, body)
	}
	if status, file := tm.download(t, tm.asset(t, body).ContentURL); status != http.StatusOK || file != sevenByFive {
		t.Errorf("photo.png's new address = %d, want its file", status)
	}
	tm.resolves(t, a, b, photo.ID)
	tm.resolves(t, linker, a)
	if items := tm.trash(t, "bob", nb); len(items) != 1 || items[0].ID != loose.ID {
		t.Errorf("the trash = %+v, want loose.png alone", items)
	}
	tm.checkTrash(t)
}

// A page in the trash resolves its links from where it was: under A, its
// [[Notes]] leads to A/Notes, nearer than B/Notes (M8/P2 design 3.7).
func TestAPageInTheTrashResolvesFromWhereItWas(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	a, b := tm.createPage(t, "alice", nb, "", "A"), tm.createPage(t, "alice", nb, "", "B")
	notesA, notesB := tm.createPage(t, "alice", nb, a, "Notes"), tm.createPage(t, "alice", nb, b, "Notes")
	src := tm.createPageWith(t, "alice", nb, a, "Src", "[[Notes]]")
	tm.send(t, nodeDeletion("alice", src), http.StatusNoContent)
	if view := tm.versionView(t, src, 1); !leadsTo(view, notesA) || leadsTo(view, notesB) {
		t.Errorf("Src's view in the trash = %s, want [[Notes]] leading to A/Notes", view)
	}
	tm.checkTrash(t)
}

// A restore goes back where it can (M8/P2 design 3.5), as the item told:
// under its parent at its order, a name a sibling took there numbered, an
// attachment's before its extension; at the root, its parent in the
// trash, or its pages too deep under a parent moved deeper meanwhile.
func TestARestoreGoesBackWhereItCan(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	p := tm.createPage(t, "alice", nb, "", "P")
	tm.createPage(t, "alice", nb, p, "First")
	c := tm.createPage(t, "alice", nb, p, "C")
	tm.createPage(t, "alice", nb, p, "Last")
	q := tm.createPage(t, "alice", nb, "", "Q")
	d := tm.createPage(t, "alice", nb, q, "D")
	pic := tm.upload(t, "alice", nb, "", "pic.png", sevenByFive)
	var eighth string
	for i := range domain.MaxDepth - 2 {
		eighth = tm.createPage(t, "alice", nb, eighth, "Level "+string(rune('A'+i)))
	}
	levelA := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE name = 'Level A'")[0]
	x := tm.createPage(t, "alice", nb, eighth, "X")
	tm.createPage(t, "alice", nb, x, "Y")
	top := tm.createPage(t, "alice", nb, "", "Top")
	for _, id := range []string{c, d, q, pic.ID, x} {
		tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)
	}
	tm.createPage(t, "alice", nb, p, "c")
	tm.upload(t, "alice", nb, "", "PIC.png", sevenByFive)
	tm.send(t, nodeMove("alice", levelA, top), http.StatusOK)
	for _, tt := range []struct {
		name, id string
		want     string
		named    string
	}{
		{"C under P, its name taken", c, p + " original", "C 2"},
		{"D, Q in the trash", d, " parent_in_trash", "D"},
		{"pic.png at the root, its name taken", pic.ID, " original", "pic 2.png"},
		{"X, too deep under its parent", x, " too_deep", "X"},
	} {
		_, item, _ := tm.trashItem(t, "alice", tt.id)
		got := tm.restored(t, "alice", tt.id)
		if got.Placed.at() != tt.want || item.RestoreTo.at() != tt.want || got.Name != tt.named || got.Renamed != (tt.named != item.Name) {
			t.Errorf("%s: restored %+v, its item back to %s; want %s as %q", tt.name, got, item.RestoreTo.at(), tt.want, tt.named)
		}
	}
	if order := queryStrings(t, tm.pool, `SELECT name FROM nodes WHERE parent_id = $1 AND deleted_at IS NULL ORDER BY sort_order, id`,
		p); fmt.Sprint(order) != "[First C 2 Last c]" {
		t.Errorf("P's children = %v, want C 2 back at its order, between First and Last", order)
	}
	if roots := queryStrings(t, tm.pool, `SELECT name FROM nodes WHERE parent_id IS NULL AND deleted_at IS NULL AND notebook_id = $1
		ORDER BY sort_order, id`, nb); fmt.Sprint(roots[len(roots)-2:]) != "[D X]" {
		t.Errorf("the roots = %v, want D and X last, in the order restored", roots)
	}
	tm.checkTrash(t)
}

// A restore past the unit's bounds is page.restore_too_large, and writes
// nothing: more than 100 pages; more than 8 MiB of their contents; more
// than 20,000 links among them, known once they are parsed. A lone page
// is never past them.
func TestARestorePastTheUnitsBoundsIsRefused(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
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
	for _, id := range []string{many, heavy, linked, lone} {
		tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)
	}
	for name, id := range map[string]string{"101 pages": many, "8 MiB and a byte": heavy, "20,002 links": linked} {
		c := nodeRestore("bob", id)
		if status, answer := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["bob"], ""); status != http.StatusUnprocessableEntity ||
			problemCode(t, answer) != "page.restore_too_large" {
			t.Errorf("restore %s = %d %s, want 422 page.restore_too_large", name, status, answer)
		}
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE kind = 'trash_restore'"); n != 0 {
		t.Errorf("%d restores' changesets, want none", n)
	}
	if got := tm.restored(t, "bob", lone); got.Name != "Lone" {
		t.Errorf("the lone page restored as %+v", got)
	}
	tm.checkTrash(t)
}

// trashedAgo moves the trash item whose root is id d further into the
// past (M8/P2 design 3.10): its group, the nodes deleted at its time from
// the root down, and what was deleted with them at that time, each at the
// same new time, as the item would be had it been deleted then.
func (tm acmeTeam) trashedAgo(t *testing.T, id string, d time.Duration) {
	t.Helper()
	ctx := context.Background()
	group := `WITH RECURSIVE r AS (SELECT deleted_at FROM nodes WHERE id = $1),
		g AS (SELECT id FROM nodes WHERE id = $1 UNION ALL
			SELECT n.id FROM nodes n JOIN g ON n.parent_id = g.id, r WHERE n.deleted_at = r.deleted_at)`
	tx, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, table := range []string{"page_contents", "page_revisions", "changeset_items", "asset_blobs", "nodes"} {
		column := "node_id"
		if table == "nodes" {
			column = "id"
		}
		if _, err := tx.Exec(ctx, group+` UPDATE `+table+` x SET deleted_at = x.deleted_at - $2::interval FROM r
			WHERE x.`+column+` IN (SELECT id FROM g) AND x.deleted_at = r.deleted_at`, id, d); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}
