package app_test

import (
	"context"
	"crypto/sha256"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// version records a version of the page id in a changeset of its own, of
// kind, by by, without recording a call: on base, nil for the page's
// creation.
func (f *fixture) version(id uuid.UUID, base *int, revision int, content string, kind domain.ChangesetKind, by uuid.UUID) app.Revision {
	at := now().Add(time.Duration(revision-100) * time.Minute)
	c := app.Changeset{ID: uuid.NewV7(), NotebookID: f.eng, Kind: kind, Client: domain.ClientWeb, By: by, At: at}
	f.store.changesets = append(f.store.changesets, c)
	sum := sha256.Sum256([]byte(content))
	r := app.Revision{ID: uuid.NewV7(), ChangesetID: c.ID, NodeID: id, Base: base, Revision: revision, Content: content, Hash: sum[:],
		ByteSize: len(content), At: at}
	f.store.revisions = append(f.store.revisions, r)
	return r
}

// history gives the page id the versions 1 to n by alice, each on the one
// before, of the contents "v1" to "vn", and the last as its content.
func (f *fixture) history(id uuid.UUID, n int) []app.Revision {
	out := make([]app.Revision, n)
	for i := range n {
		var base *int
		if i > 0 {
			base = &out[i-1].Revision
		}
		out[i] = f.version(id, base, i+1, "v"+strconv.Itoa(i+1), domain.ChangesetEdit, f.alice)
	}
	c := f.store.contents[id]
	c.Content, c.Revision, c.Hash, c.ByteSize = out[n-1].Content, n, out[n-1].Hash, out[n-1].ByteSize
	f.store.contents[id] = c
	return out
}

// trash moves the page n to the trash, deleted an hour ago.
func (f *fixture) trash(n domain.Node) {
	at := now().Add(-time.Hour)
	f.store.trash[n.ID] = app.AnyNode{Node: n, DeletedAt: &at}
	delete(f.store.nodes, n.ID)
	delete(f.store.contents, n.ID)
}

func (f *fixture) versionReads() app.VersionReads {
	return app.VersionReads{Notebooks: f.notebooks, Nodes: f.store, Versions: f.store, Names: f.names, Auth: f.auth, Clock: f.clock}
}

// countingNames is fakeNames that counts its reads.
type countingNames struct {
	fakeNames
	reads *int
}

func (n countingNames) DisplayNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	*n.reads++
	return n.fakeNames.DisplayNames(ctx, ids)
}

func (f *fixture) listVersions(id uuid.UUID, limit int, cursor string) (app.VersionList, error) {
	var c *string
	if cursor != "" {
		c = &cursor
	}
	return app.NewListVersions(f.versionReads()).Execute(f.asAlice(), id, &limit, c)
}

func revisionsOf(versions []domain.Version) []int {
	out := make([]int, len(versions))
	for i, v := range versions {
		out[i] = v.Revision
	}
	return out
}

// A page's versions list newer-first, a page at a time, without a
// transaction: the node alive or in the trash, its notebook's workspace,
// the decision on page.history, the versions; each with its changeset and
// its actor's name, the names of a page read at once. The cursor is the
// last revision listed; the last page has none.
func TestListVersionsPagesNewerFirst(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionHistory)
	bob := uuid.NewV7()
	f.names[f.alice], f.names[bob] = "Alice", "Bob"
	n := f.page("Notes", nil, 0)
	f.history(n.ID, 6)
	for _, i := range []int{1, 2, 4} {
		f.store.changesets[i].By = bob
	}
	reads := 0
	list := app.NewListVersions(app.VersionReads{Notebooks: f.notebooks, Nodes: f.store, Versions: f.store,
		Names: countingNames{f.names, &reads}, Auth: f.auth, Clock: f.clock})
	var pages [][]int
	var names []string
	cursor := ""
	for range 4 {
		f.rec.calls = nil
		limit := 2
		var c *string
		if cursor != "" {
			c = &cursor
		}
		l, err := list.Execute(f.asAlice(), n.ID, &limit, c)
		if err != nil {
			t.Fatal(err)
		}
		pages = append(pages, revisionsOf(l.Versions))
		for _, v := range l.Versions {
			names = append(names, v.Changeset.ByName)
			if v.Changeset.Kind != domain.ChangesetEdit || v.Editing {
				t.Errorf("version %+v, want an edit, not being edited", v)
			}
		}
		want := []string{"FindAnyNode", "WorkspaceOf", "Authorize page.history", "ListVersions"}
		if cursor == "" {
			want = append(want, "AliveChangesetOf")
		}
		if !slices.Equal(f.rec.calls, want) {
			t.Errorf("calls = %v, want %v, outside a transaction", f.rec.calls, want)
		}
		if cursor = l.NextCursor; cursor == "" {
			break
		}
	}
	if !slices.EqualFunc(pages, [][]int{{6, 5}, {4, 3}, {2, 1}}, slices.Equal) {
		t.Errorf("pages = %v, want [6 5] [4 3] [2 1], the last with no cursor", pages)
	}
	if want := []string{"Alice", "Bob", "Alice", "Bob", "Bob", "Alice"}; !slices.Equal(names, want) || reads != 3 {
		t.Errorf("names %v in %d reads, want %v, one read a page", names, reads, want)
	}
}

// A page in the trash has its history, read as an alive page's.
func TestListVersionsOfAPageInTheTrash(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionHistory)
	n := f.page("Notes", nil, 0)
	f.history(n.ID, 2)
	f.trash(n)
	l, err := f.listVersions(n.ID, 50, "")
	if err != nil || !slices.Equal(revisionsOf(l.Versions), []int{2, 1}) {
		t.Errorf("versions = %v, %v; want 2 and 1", revisionsOf(l.Versions), err)
	}
}

// A cursor that does not decode, or names no revision a version may have,
// is 400 bad_request before anything is read; then the page's 404, of a
// page that is no page, not there, of a notebook deleted or not seen; then
// the limit's 422.
func TestListVersionsAnswersItsCodesInOrder(t *testing.T) {
	f := newFixture()
	n := f.page("Notes", nil, 0)
	f.history(n.ID, 2)
	for _, below := range []int{0, -1, domain.MaxRevision + 1} {
		cursor, err := shared.EncodeCursor(domain.VersionCursor{Below: below})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.listVersions(uuid.NewV7(), 0, cursor); codeOf(err) != "bad_request" || len(f.rec.calls) != 0 {
			t.Errorf("a cursor below %d = %q after %v, want bad_request before any read", below, codeOf(err), f.rec.calls)
		}
	}
	if _, err := f.listVersions(n.ID, 0, "x"); codeOf(err) != "bad_request" || len(f.rec.calls) != 0 {
		t.Errorf("a cursor that does not decode = %q after %v, want bad_request before any read", codeOf(err), f.rec.calls)
	}
	asset := f.page("photo.png", nil, 1)
	asset.Kind = domain.KindAsset
	f.store.nodes[asset.ID] = asset
	for _, id := range []uuid.UUID{uuid.NewV7(), asset.ID, n.ID} {
		if _, err := f.listVersions(id, 0, ""); codeOf(err) != "page.not_found" {
			t.Errorf("listVersions not granted = %q, want page.not_found", codeOf(err))
		}
	}
	f.grant(domain.ActionHistory)
	if _, err := f.listVersions(n.ID, 0, ""); codeOf(err) != "validation_failed limit" {
		t.Errorf("a limit of 0 = %q, want validation_failed on limit", codeOf(err))
	}
	if _, err := f.listVersions(n.ID, 101, ""); codeOf(err) != "validation_failed limit" {
		t.Errorf("a limit of 101 = %q, want validation_failed on limit", codeOf(err))
	}
	delete(f.notebooks.workspaces, f.eng)
	if _, err := f.listVersions(n.ID, 0, "x"); codeOf(err) != "bad_request" {
		t.Errorf("a bad cursor of a deleted notebook's page = %q, want bad_request first", codeOf(err))
	}
	if _, err := f.listVersions(n.ID, 50, ""); codeOf(err) != "page.not_found" {
		t.Errorf("a deleted notebook's page = %q, want page.not_found", codeOf(err))
	}
}

// The newest version is being edited while the edit session whose
// changeset it is is alive: not once the session expired, not while an
// alive session has not written or wrote another version, and no older
// version.
func TestListVersionsMarksTheVersionBeingEdited(t *testing.T) {
	for _, tt := range []struct {
		name    string
		expires time.Duration
		written int
		want    bool
	}{
		{"an alive session's", time.Minute, 3, true},
		{"an expired session's", -time.Minute, 3, false},
		{"an alive session not written", time.Minute, 0, false},
		{"an alive session of an older version", time.Minute, 2, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionHistory)
			n := f.page("Notes", nil, 0)
			versions := f.history(n.ID, 3)
			s := f.session(n.ID, f.alice, now().Add(tt.expires))
			if tt.written > 0 {
				s.ChangesetID, s.Revision = versions[tt.written-1].ChangesetID, tt.written
				f.store.sessions[s.ID] = s
			}
			l, err := f.listVersions(n.ID, 50, "")
			if err != nil {
				t.Fatal(err)
			}
			if l.Versions[0].Editing != tt.want || l.Versions[1].Editing || l.Versions[2].Editing {
				t.Errorf("editing = %v %v %v, want %v for the newest alone", l.Versions[0].Editing, l.Versions[1].Editing,
					l.Versions[2].Editing, tt.want)
			}
		})
	}
}

// A version is addressed by any revision it holds: an edit session's
// version holds those from its base on, and is answered with its own
// revision; a revision no version holds is page.version_not_found, one
// below the first, past the last, or in a gap the chain of versions would
// never have.
func TestGetVersionFindsTheVersionHoldingARevision(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionHistory)
	f.names[f.alice] = "Alice"
	n := f.page("Notes", nil, 0)
	first := f.version(n.ID, nil, 1, "one", domain.ChangesetEdit, f.alice)
	session := f.version(n.ID, &first.Revision, 4, "four", domain.ChangesetEdit, f.alice)
	six := 6
	afterAGap := f.version(n.ID, &six, 7, "seven", domain.ChangesetEdit, f.alice)
	get := app.NewGetVersion(f.versionReads())
	for revision, want := range map[int]app.Revision{1: first, 2: session, 3: session, 4: session, 7: afterAGap} {
		f.rec.calls = nil
		v, err := get.Execute(f.asAlice(), n.ID, revision)
		if err != nil {
			t.Fatal(err)
		}
		if v.Version.ID != want.ID || v.Version.Revision != want.Revision || v.Content != want.Content ||
			v.Version.Changeset.ByName != "Alice" {
			t.Errorf("version %d = %+v, want %q at %d by Alice", revision, v, want.Content, want.Revision)
		}
		if !slices.Equal(f.rec.calls, []string{"FindAnyNode", "WorkspaceOf", "Authorize page.history", "FindVersion",
			"AliveChangesetOf"}) {
			t.Errorf("calls = %v, want the page, the decision, the version with its content, its session, outside a transaction",
				f.rec.calls)
		}
	}
	for _, revision := range []int{0, -1, 5, 6, 8, domain.MaxRevision + 1} {
		if _, err := get.Execute(f.asAlice(), n.ID, revision); codeOf(err) != "page.version_not_found" {
			t.Errorf("version %d = %q, want page.version_not_found", revision, codeOf(err))
		}
	}
	f.trash(n)
	if v, err := get.Execute(f.asAlice(), n.ID, 1); err != nil || v.Content != "one" {
		t.Errorf("a version of a page in the trash = %+v, %v; want it", v, err)
	}
	if _, err := get.Execute(f.asAlice(), uuid.NewV7(), 1); codeOf(err) != "page.not_found" {
		t.Errorf("a version of no page = %q, want page.not_found", codeOf(err))
	}
}

// A version is being edited while it is its page's newest and its edit
// session is alive: a session's older version is not.
func TestGetVersionMarksOnlyTheNewestVersionBeingEdited(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionHistory)
	n := f.page("Notes", nil, 0)
	versions := f.history(n.ID, 3)
	s := f.session(n.ID, f.alice, now().Add(time.Minute))
	get := app.NewGetVersion(f.versionReads())
	for _, tt := range []struct {
		of, asked int
		want      bool
	}{{3, 3, true}, {2, 2, false}, {2, 3, false}} {
		s.ChangesetID = versions[tt.of-1].ChangesetID
		f.store.sessions[s.ID] = s
		v, err := get.Execute(f.asAlice(), n.ID, tt.asked)
		if err != nil {
			t.Fatal(err)
		}
		if v.Version.Editing != tt.want {
			t.Errorf("version %d with the session's changeset version %d's: editing %v, want %v", tt.asked, tt.of,
				v.Version.Editing, tt.want)
		}
	}
}

// A version's reading view is rendered as a page's, for this page at the
// version's own revision, within the parse budget; a page in the trash's
// for a page deleted, whose links resolve from where it was (M8/P2 design
// 3.7).
func TestGetVersionViewRendersTheVersion(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionHistory)
	n := f.page("Notes", nil, 0)
	first := f.version(n.ID, nil, 1, "one", domain.ChangesetEdit, f.alice)
	f.version(n.ID, &first.Revision, 3, "three", domain.ChangesetEdit, f.alice)
	f.trash(n)
	md := &fakeMarkdown{recorder: f.rec, expires: now().Add(time.Hour)}
	v, err := app.NewGetVersionView(f.versionReads(), md, f.budget).Execute(f.asAlice(), n.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if v != (app.ReadingView{HTML: "<p>three</p>", Revision: 3, Expires: md.expires}) {
		t.Errorf("view = %+v, want version 3 rendered", v)
	}
	if !slices.Equal(f.rec.calls, []string{"FindAnyNode", "WorkspaceOf", "Authorize page.history", "FindVersion", "Take 5", "Render",
		"Release 5"}) {
		t.Errorf("calls = %v, want the page, the decision, the version with its content, rendered within the budget", f.rec.calls)
	}
	if !slices.Equal(md.pages, []app.PageRef{{NotebookID: f.eng, PageID: n.ID, Revision: 3, Deleted: true}}) {
		t.Errorf("rendered for %v, want the page in the trash at revision 3", md.pages)
	}
	alive := f.page("Alive", nil, 1)
	f.version(alive.ID, nil, 1, "one", domain.ChangesetEdit, f.alice)
	if _, err := app.NewGetVersionView(f.versionReads(), md, f.budget).Execute(f.asAlice(), alive.ID, 1); err != nil ||
		md.pages[1] != (app.PageRef{NotebookID: f.eng, PageID: alive.ID, Revision: 1}) {
		t.Errorf("rendered %v for %v, want the page alive at revision 1", err, md.pages)
	}
	if _, err := app.NewGetVersionView(f.versionReads(), md, f.budget).Execute(f.asAlice(), n.ID, 4); codeOf(err) !=
		"page.version_not_found" {
		t.Errorf("view of version 4 = %q, want page.version_not_found", codeOf(err))
	}
	f.budget.err = shared.ServerBusy(time.Second)
	if _, err := app.NewGetVersionView(f.versionReads(), md, f.budget).Execute(f.asAlice(), n.ID, 1); codeOf(err) != "server_busy" {
		t.Errorf("view with the budget full = %q, want server_busy", codeOf(err))
	}
}

// restore runs restorePageVersion on the page id as alice from the web.
func (f *fixture) restore(id uuid.UUID, revision, base int) (app.VersionRestored, error) {
	return app.NewRestoreVersion(f.versionReads(), f.writer(), f.parser(), f.logger()).Execute(f.asAlice(), id, revision, base,
		domain.ClientWeb)
}

// Restoring a version writes its content as the page's, in a changeset of
// its own of the kind version_restore, a version on the page's revision:
// decided unlocked, parsed within the budget, then the unit.
func TestRestoreVersionWritesTheVersionsContent(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionHistory, domain.ActionRestoreVersion)
	g := &guard{recorder: f.rec}
	f.guards = []app.WriteGuard{g}
	n := f.page("Notes", nil, 0)
	f.history(n.ID, 3)
	f.rec.calls = nil
	r, err := f.restore(n.ID, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	c := f.store.contents[n.ID]
	last := f.store.revisions[len(f.store.revisions)-1]
	if r.Revision != 4 || c.Content != "v1" || c.Revision != 4 || last.Revision != 4 || *last.Base != 3 || last.Content != "v1" ||
		last.ChangesetID != r.ChangesetID {
		t.Errorf("restored %+v; content %q at %d, last version %+v; want v1 at revision 4, a version on 3", r, c.Content, c.Revision, last)
	}
	cs := f.store.changesets[len(f.store.changesets)-1]
	if cs.ID != r.ChangesetID || cs.Kind != domain.ChangesetVersionRestore || cs.RevertsID != (uuid.UUID{}) {
		t.Errorf("changeset %+v, want a version_restore", cs)
	}
	if len(g.steps) != 1 || g.steps[0].Kind != domain.ChangesetVersionRestore || g.steps[0].Operation != domain.OpContent {
		t.Errorf("the guard saw %+v, want a content write of a version_restore", g.steps)
	}
	want := []string{"FindAnyNode", "WorkspaceOf", "Authorize page.history", "FindVersion", "WorkspaceOf",
		"Authorize page.restore_version", "Take 2", "Facts", "KeepFacts 2", "WorkspaceOf", "ShareWorkspace in tx",
		"ShareNotebook in tx", "Authorize page.restore_version in tx", "LockContent in tx", "FindNodeIn in tx",
		"GuardWrite content in tx", "CreateChangeset in tx", "WriteContent in tx", "RecordRevision in tx"}
	if !slices.Equal(f.rec.calls[:len(want)], want) || f.rec.calls[len(f.rec.calls)-1] != "Release 2" {
		t.Errorf("calls = %v, want them to begin %v and end with the release", f.rec.calls, want)
	}
	logged := f.logs.String()
	if !strings.Contains(logged, "page content restored") || !strings.Contains(logged, "restored_revision=1") ||
		!strings.Contains(logged, "revision=4") || strings.Contains(logged, "v1") {
		t.Errorf("logged %q, want the restore's ids and revisions, not the content", logged)
	}
}

// A version whose content the page holds already writes nothing, whatever
// the base, and while an edit session holds the page's lock: the page's
// revision, no changeset, nothing logged. The content is parsed first:
// with the budget full, server_busy.
func TestRestoringTheCurrentContentWritesNothing(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionHistory, domain.ActionRestoreVersion)
	f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: domain.Locked(uuid.NewV7(), uuid.NewV7(), "Bob")}}
	n := f.page("Notes", nil, 0)
	versions := f.history(n.ID, 3)
	f.version(n.ID, &versions[2].Revision, 4, "v3", domain.ChangesetEdit, f.alice)
	c := f.store.contents[n.ID]
	c.Revision = 4
	f.store.contents[n.ID] = c
	before := len(f.store.changesets)
	for _, base := range []int{4, 1} {
		r, err := f.restore(n.ID, 3, base)
		if err != nil || r != (app.VersionRestored{Revision: 4}) || len(f.store.changesets) != before || f.logs.Len() != 0 {
			t.Errorf("restore on %d = %+v, %v; %d changesets, logged %q; want revision 4, nothing written", base, r, err,
				len(f.store.changesets), f.logs.String())
		}
	}
	f.budget.err = shared.ServerBusy(time.Second)
	if _, err := f.restore(n.ID, 3, 4); codeOf(err) != "server_busy" {
		t.Errorf("restore with the budget full = %q, want server_busy", codeOf(err))
	}
}

// A restore's codes come in M8 design 4.14's order: page.not_found (no
// page, an attachment, a page in the trash, a notebook not seen), then
// page.version_not_found, then forbidden, then server_busy, then
// page.revision_mismatch, then page.locked. Each case has every later
// fault too.
func TestRestoreVersionAnswersItsCodesInOrder(t *testing.T) {
	locked := domain.Locked(uuid.NewV7(), uuid.NewV7(), "Bob")
	type faults struct{ notSeen, trashed, noVersion, reader, busy, mismatch, locked bool }
	for _, tt := range []struct {
		name string
		f    faults
		want string
	}{
		{"a notebook not seen", faults{true, false, true, true, true, true, true}, "page.not_found"},
		{"a page in the trash", faults{false, true, true, true, true, true, true}, "page.not_found"},
		{"no version", faults{false, false, true, true, true, true, true}, "page.version_not_found"},
		{"a reader", faults{false, false, false, true, true, true, true}, "forbidden"},
		{"the budget full", faults{false, false, false, false, true, true, true}, "server_busy"},
		{"a base not the page's", faults{false, false, false, false, false, true, true}, "page.revision_mismatch"},
		{"the page locked", faults{false, false, false, false, false, false, true}, "page.locked"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			n := f.page("Notes", nil, 0)
			f.history(n.ID, 3)
			if !tt.f.notSeen {
				f.grant(domain.ActionHistory, domain.ActionRestoreVersion)
			}
			if tt.f.trashed {
				f.trash(n)
			}
			revision, base := 1, 3
			if tt.f.noVersion {
				revision = 9
			}
			if tt.f.reader {
				delete(f.auth.grants, domain.ActionRestoreVersion)
				f.auth.forbidden[domain.ActionRestoreVersion] = true
			}
			if tt.f.busy {
				f.budget.err = shared.ServerBusy(time.Second)
			}
			if tt.f.mismatch {
				base = 2
			}
			if tt.f.locked {
				f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: locked}}
			}
			before := len(f.store.revisions)
			if _, err := f.restore(n.ID, revision, base); codeOf(err) != tt.want || len(f.store.revisions) != before {
				t.Errorf("restore = %v, %d versions; want %s, none written", err, len(f.store.revisions)-before, tt.want)
			}
		})
	}
	f := newFixture()
	f.grant(domain.ActionHistory, domain.ActionRestoreVersion)
	asset := f.page("photo.png", nil, 1)
	asset.Kind = domain.KindAsset
	f.store.nodes[asset.ID] = asset
	for _, id := range []uuid.UUID{uuid.NewV7(), asset.ID} {
		if _, err := f.restore(id, 1, 1); codeOf(err) != "page.not_found" {
			t.Errorf("restore of no page = %q, want page.not_found", codeOf(err))
		}
	}
}
