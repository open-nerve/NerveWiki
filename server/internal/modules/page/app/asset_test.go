package app_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// upload is the action the asset module decides an attachment's creation
// on: the page module takes it as given.
const upload shared.Action = "asset.upload"

// newAsset is an attachment of eng from the web, named name, under parent.
func (f *fixture) newAsset(name string, parent *uuid.UUID) app.NewAsset {
	return app.NewAsset{NotebookID: f.eng, ParentID: parent, Name: name, Action: upload, Client: domain.ClientWeb,
		Meta: app.AssetMeta{MIME: "image/png", Bytes: 3, SHA256: bytes.Repeat([]byte{7}, 32)}}
}

// createAsset creates a as alice; after records its call and answers
// err.
func (f *fixture) createAsset(a app.NewAsset, err error) (domain.Node, error) {
	return app.NewAssetWrites(f.writer(), f.store).CreateAsset(f.asAlice(), a, func(ctx context.Context, _ domain.Node) error {
		f.rec.record(ctx, "after")
		return err
	})
}

// checkAsset checks a as alice.
func (f *fixture) checkAsset(a app.NewAsset) error {
	return app.NewAssetWrites(f.writer(), f.store).Check(f.asAlice(), a)
}

// asset adds an attachment of eng named name under parent to the store.
func (f *fixture) asset(name string, parent *uuid.UUID, order float64) domain.Node {
	n := f.page(name, parent, order)
	n.Kind = domain.KindAsset
	f.store.nodes[n.ID] = n
	delete(f.store.contents, n.ID)
	return n
}

// An attachment's unit locks, decides on the action it is given, checks
// the name, the parent and the siblings, has the guard see the file, then
// writes the node, its name trimmed and its key folded, its item in a
// changeset of the client's, and runs the participants; after follows, in
// the transaction, then the observers.
func TestCreateAssetRunsInAUnitThatChangesTheTree(t *testing.T) {
	f := newFixture()
	f.grant(upload)
	g, p, o := &guard{recorder: f.rec}, &participant{recorder: f.rec}, &observer{recorder: f.rec}
	f.guards, f.partakers, f.observers = []app.WriteGuard{g}, []app.Participant{p}, []app.PageObserver{o}
	parent := f.page("Parent", nil, 0)
	a := f.newAsset(" Photo.PNG ", &parent.ID)
	a.Client = domain.ClientAPI
	n, err := f.createAsset(a, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"WorkspaceOf", "ShareWorkspace in tx", "LockNotebook in tx", "Authorize asset.upload in tx", "FindNodeIn in tx",
		"Children in tx", "GuardWrite create in tx", "CreateChangeset in tx", "CreateNode in tx", "RecordItem in tx",
		"Participate create in tx", "after in tx", "PagesChanged in tx"}
	if !slices.Equal(f.rec.calls, want) {
		t.Errorf("calls = %v, want %v", f.rec.calls, want)
	}
	if len(g.steps) != 1 || g.steps[0].Asset == nil || g.steps[0].Asset.MIME != "image/png" || g.steps[0].Asset.Bytes != 3 ||
		!bytes.Equal(g.steps[0].Asset.SHA256, a.Meta.SHA256) {
		t.Errorf("the guard saw %+v, want the attachment's file", g.steps)
	}
	if len(o.events) != 1 || len(o.events[0].Changes) != 1 || o.events[0].Changes[0].Revision != 0 || o.events[0].Changes[0].Facts != nil ||
		o.events[0].Changes[0].Before != nil || o.events[0].Changes[0].After.Name != "Photo.PNG" {
		t.Errorf("the observer saw %+v, want the attachment created without a revision or facts", o.events)
	}
	if n.Kind != domain.KindAsset || n.Name != "Photo.PNG" || n.NameKey != shared.TitleKey("Photo.PNG") || n.NotebookID != f.eng ||
		*n.ParentID != parent.ID || n.CreatedBy != f.alice || !n.CreatedAt.Equal(now()) || n.UpdatedBy != f.alice ||
		!n.UpdatedAt.Equal(now()) || f.store.nodes[n.ID] != n {
		t.Errorf("node = %+v, want Photo.PNG, keyed photo.png, an attachment of eng under Parent by alice at %v", n, now())
	}
	if _, ok := f.store.contents[n.ID]; ok || len(f.store.revisions) != 0 {
		t.Errorf("content %+v, versions %+v; want none for an attachment", f.store.contents[n.ID], f.store.revisions)
	}
	if item := f.store.items[n.ID]; len(f.store.changesets) != 1 || f.store.changesets[0].Client != domain.ClientAPI ||
		item.ChangesetID != f.store.changesets[0].ID || item.Change.After == nil {
		t.Errorf("item = %+v of %+v, want the creation in the unit's changeset, of the api", item, f.store.changesets)
	}
	if f.auth.targets[0] != (shared.Target{WorkspaceID: f.acme, NotebookID: f.eng}) {
		t.Errorf("decided on %+v, want acme's eng", f.auth.targets[0])
	}
}

// An attachment goes after every sibling, a page's and an attachment's;
// after a last sibling with no room after it, the siblings are numbered
// again.
func TestCreateAssetPlacesItLast(t *testing.T) {
	f := newFixture()
	f.grant(upload)
	f.page("B", nil, 7)
	f.asset("a.png", nil, 2)
	n, err := f.createAsset(f.newAsset("c.png", nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	if n.SortOrder <= 7 {
		t.Errorf("order = %v, want it after 7, the last sibling's", n.SortOrder)
	}

	f = newFixture()
	f.grant(upload)
	f.page("B", nil, 1<<53)
	if _, err := f.createAsset(f.newAsset("c.png", nil), nil); err != nil {
		t.Fatal(err)
	}
	if got := orderedNames(t, f); !slices.Equal(got, []string{"B", "c.png"}) || !f.called("SetSortOrder in tx") {
		t.Errorf("roots = %v, renumbered %v; want B, c.png, renumbered", got, f.called("SetSortOrder in tx"))
	}
}

// A read of the parent or of the siblings that fails is Check's and
// CreateAsset's failure, not a refusal nor a pass.
func TestCreateAssetAnswersAReadThatFails(t *testing.T) {
	failure := errors.New("connection reset")
	for _, read := range []string{"FindNodeIn", "Children"} {
		t.Run(read, func(t *testing.T) {
			f := newFixture()
			f.grant(upload)
			parent := f.page("Parent", nil, 0)
			f.store.errs = map[string]error{read: failure}
			a := f.newAsset("a.png", &parent.ID)
			if err := f.checkAsset(a); !errors.Is(err, failure) {
				t.Errorf("Check() = %v, want the read's failure", err)
			}
			if _, err := f.createAsset(a, nil); !errors.Is(err, failure) {
				t.Errorf("CreateAsset() = %v, want the read's failure", err)
			}
		})
	}
}

// An attachment is no level: a page of the tenth level holds one.
func TestCreateAssetUnderTheDeepestPage(t *testing.T) {
	f := newFixture()
	f.grant(upload)
	var parent *uuid.UUID
	for i := range domain.MaxDepth {
		n := f.page("Level "+string(rune('A'+i)), parent, 0)
		parent = &n.ID
	}
	if _, err := f.createAsset(f.newAsset("photo.png", parent), nil); err != nil {
		t.Errorf("an attachment under the tenth level = %v, want it created", err)
	}
}

// Check answers what CreateAsset answers, in the same order, unlocked
// and before it writes; the unit's guard and after come only in
// CreateAsset, after every other check, and roll the unit back.
func TestCreateAssetAndCheckAnswerTheirCodesInOrder(t *testing.T) {
	refusal := shared.NewError(shared.KindConflict, "page.locked", "Locked.")
	afterFailed := errors.New("the row failed")
	for _, tt := range []struct {
		name     string
		setup    func(f *fixture) app.NewAsset
		after    error
		want     string
		unitOnly bool // Check passes it
	}{
		{"an unknown notebook before the other values", func(f *fixture) app.NewAsset {
			a := f.newAsset("a.md", nil)
			a.NotebookID = uuid.NewV7()
			return a
		}, nil, "notebook.not_found", false},
		{"a notebook the caller cannot see", func(f *fixture) app.NewAsset {
			return f.newAsset("a.md", nil)
		}, nil, "notebook.not_found", false},
		{"a reader before the values", func(f *fixture) app.NewAsset {
			f.auth.forbidden[upload] = true
			return f.newAsset("a.md", nil)
		}, nil, "forbidden", false},
		{"the name before the parent", func(f *fixture) app.NewAsset {
			f.grant(upload)
			missing := uuid.NewV7()
			return f.newAsset("a:b.png", &missing)
		}, nil, "validation_failed name", false},
		{"a name ending with .md", func(f *fixture) app.NewAsset {
			f.grant(upload)
			return f.newAsset("notes.MD", nil)
		}, nil, "validation_failed name", false},
		{"a parent of no page of the notebook", func(f *fixture) app.NewAsset {
			f.grant(upload)
			missing := uuid.NewV7()
			return f.newAsset("a.png", &missing)
		}, nil, "validation_failed parent_id", false},
		{"a parent that is an attachment", func(f *fixture) app.NewAsset {
			f.grant(upload)
			parent := f.asset("b.png", nil, 0)
			return f.newAsset("a.png", &parent.ID)
		}, nil, "validation_failed parent_id", false},
		{"a parent before a name taken", func(f *fixture) app.NewAsset {
			f.grant(upload)
			f.page("A.png", nil, 0)
			missing := uuid.NewV7()
			return f.newAsset("a.png", &missing)
		}, nil, "validation_failed parent_id", false},
		{"a name a page holds", func(f *fixture) app.NewAsset {
			f.grant(upload)
			f.page("A.png", nil, 0)
			return f.newAsset("a.png", nil)
		}, nil, "page.title_taken", false},
		{"a name an attachment holds", func(f *fixture) app.NewAsset {
			f.grant(upload)
			parent := f.page("Parent", nil, 0)
			f.asset("a.PNG", &parent.ID, 0)
			return f.newAsset("a.png", &parent.ID)
		}, nil, "page.title_taken", false},
		{"a name taken before the guard", func(f *fixture) app.NewAsset {
			f.grant(upload)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			f.page("A.png", nil, 0)
			return f.newAsset("a.png", nil)
		}, nil, "page.title_taken", false},
		{"the guard", func(f *fixture) app.NewAsset {
			f.grant(upload)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			return f.newAsset("a.png", nil)
		}, nil, "page.locked", true},
		{"after's error last", func(f *fixture) app.NewAsset {
			f.grant(upload)
			return f.newAsset("a.png", nil)
		}, afterFailed, afterFailed.Error(), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			a := tt.setup(f)
			want := tt.want
			if tt.unitOnly {
				want = ""
			}
			if got := codeOf(f.checkAsset(a)); got != want {
				t.Errorf("Check = %q, want %q", got, want)
			}
			if slices.ContainsFunc(f.rec.calls, func(c string) bool { return strings.HasSuffix(c, " in tx") }) {
				t.Errorf("Check made calls %v, want none in a transaction", f.rec.calls)
			}
			f.rec.calls = nil
			before := len(f.store.nodes)
			n, err := f.createAsset(a, tt.after)
			if got := codeOf(err); got != tt.want {
				t.Errorf("CreateAsset = %q, want %q", got, tt.want)
			}
			if n != (domain.Node{}) || !tt.unitOnly && (len(f.store.nodes) != before || f.called("after in tx")) {
				t.Errorf("a refused CreateAsset answered %+v, wrote %d nodes, calls %v; want nothing, after not run", n,
					len(f.store.nodes)-before, f.rec.calls)
			}
			if tt.unitOnly && !f.tx.rolledBack {
				t.Error("the unit was not rolled back")
			}
		})
	}
}

// An attachment's new name follows an attachment's rules, a page's
// follows a page's: a page may end with ".md".
func TestRenameNodeChecksTheNameByTheNodesKind(t *testing.T) {
	for _, tt := range []struct {
		name, old, new, want string
		asset                bool
	}{
		{"an attachment's new extension", "photo.png", "cover.jpeg", "", true},
		{"an attachment named .md", "photo.png", "photo.md", "validation_failed name", true},
		{"an attachment losing its extension", "photo.png", "photo", "validation_failed name", true},
		{"an attachment without an extension", "README", "LICENSE", "", true},
		{"a page named .md", "Notes", "Notes.md", "", false},
		{"a page losing its extension", "Notes.txt", "Notes", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRename)
			n := f.page(tt.old, nil, 0)
			if tt.asset {
				n = f.asset(tt.old, nil, 1)
			}
			renamed, err := f.rename(n.ID, tt.new)
			if got := codeOf(err); got != tt.want {
				t.Errorf("rename %q to %q = %q, want %q", tt.old, tt.new, got, tt.want)
			}
			if err == nil && renamed.Name != tt.new {
				t.Errorf("renamed = %+v, want %q", renamed, tt.new)
			}
		})
	}
}

// A page's subtree is as deep as its pages: one of the ninth level with
// attachments under its child moves under a page of the eighth, and an
// attachment moves under a page of the tenth.
func TestMoveNodeCountsPagesOnly(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionMove)
	var parent *uuid.UUID
	var levels []domain.Node
	for i := range domain.MaxDepth {
		n := f.page("Level "+string(rune('A'+i)), parent, 0)
		levels = append(levels, n)
		parent = &n.ID
	}
	moved := f.page("Moved", nil, 1)
	child := f.page("Child", &moved.ID, 0)
	f.asset("photo.png", &child.ID, 0)
	if _, err := f.move(moved.ID, app.Destination{ParentID: &levels[domain.MaxDepth-3].ID}); err != nil {
		t.Errorf("a page two levels high, attachments under it, to the ninth = %v, want it moved", err)
	}
	photo := f.asset("other.png", nil, 2)
	if _, err := f.move(photo.ID, app.Destination{ParentID: &levels[domain.MaxDepth-1].ID}); err != nil {
		t.Errorf("an attachment under the tenth level = %v, want it moved", err)
	}
	if _, err := f.move(moved.ID, app.Destination{ParentID: &levels[domain.MaxDepth-2].ID}); codeOf(err) != "page.too_deep" {
		t.Errorf("a page two levels high to the tenth = %v, want page.too_deep", err)
	}
}
