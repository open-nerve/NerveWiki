package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// imports are the fixture's import writes, deciding on the create action.
func (f *fixture) imports() *app.ImportWrites {
	return app.NewImportWrites(f.writer(), f.parser(), f.md)
}

// importAs runs an import's unit of eng from the web, merged into into
// when it is set.
func (f *fixture) importAs(into uuid.UUID, do func(ctx context.Context, u *app.ImportUnit) error) (uuid.UUID, error) {
	return f.imports().Import(f.asAlice(), app.ImportSpec{NotebookID: f.eng, Action: domain.ActionCreate, Client: domain.ClientWeb,
		Changeset: into}, do)
}

// childNames are the names of parent's children in eng, in order.
func (f *fixture) childNames(parent *uuid.UUID) []string {
	var kids []domain.Node
	for _, n := range f.store.nodes {
		if n.NotebookID == f.eng && (n.ParentID == nil) == (parent == nil) && (parent == nil || *n.ParentID == *parent) {
			kids = append(kids, n)
		}
	}
	slices.SortFunc(kids, func(a, b domain.Node) int {
		switch {
		case a.SortOrder < b.SortOrder:
			return -1
		case a.SortOrder > b.SortOrder:
			return 1
		}
		return 0
	})
	out := make([]string, len(kids))
	for i, n := range kids {
		out[i] = n.Name
	}
	return out
}

// An import's unit creates pages and attachments last among their
// siblings, each through the guards, in one changeset of the import kind
// that a later unit merges into; a page's content is written at revision
// 1 with its facts, an attachment's after runs in the unit.
func TestAnImportsUnitsCreatePagesAndAttachments(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	g := &guard{recorder: f.rec}
	f.guards = []app.WriteGuard{g}
	f.page("Existing", nil, 0)
	parsed, err := f.imports().Parse(f.asAlice(), "see [[A]] and [[B]]")
	if err != nil || parsed.Links != 2 {
		t.Fatalf("Parse = %+v, %v; want 2 links", parsed, err)
	}
	var page, asset domain.Node
	var after int
	first, err := f.importAs(uuid.UUID{}, func(ctx context.Context, u *app.ImportUnit) error {
		var err error
		if page, err = u.CreatePage(ctx, app.ImportedPage{Name: "Notes", Content: "see [[A]] and [[B]]", Parsed: parsed}); err != nil {
			return err
		}
		asset, err = u.CreateAsset(ctx, app.ImportedAsset{ParentID: &page.ID, Name: "photo.png", Meta: app.AssetMeta{MIME: "image/png", Bytes: 3}},
			func(ctx context.Context, n domain.Node) error {
				after++
				if n.ID == (uuid.UUID{}) || !f.tx.InTx(ctx) {
					return errors.New("after outside the unit")
				}
				return nil
			})
		return err
	})
	parsed.Release()
	if err != nil {
		t.Fatal(err)
	}
	if f.budget.held != 0 {
		t.Errorf("the budget holds %d bytes once released, want 0", f.budget.held)
	}
	if len(f.store.changesets) != 1 || f.store.changesets[0].ID != first || f.store.changesets[0].Kind != domain.ChangesetImport {
		t.Fatalf("changesets = %+v, want one of the import kind, %s", f.store.changesets, first)
	}
	if got := f.childNames(nil); !slices.Equal(got, []string{"Existing", "Notes"}) {
		t.Errorf("the root's children = %v, want the import's last", got)
	}
	if c := f.store.contents[page.ID]; c.Revision != 1 || c.Content != "see [[A]] and [[B]]" {
		t.Errorf("the page's content = %+v, want revision 1 of it", c)
	}
	if asset.Kind != domain.KindAsset || *asset.ParentID != page.ID || after != 1 {
		t.Errorf("the attachment = %+v, after ran %d times; want it under the page, after once", asset, after)
	}
	if len(g.steps) != 2 || g.steps[0].Changes[0].Facts != "see [[A]] and [[B]]" || g.steps[1].Asset == nil || g.steps[1].Asset.MIME != "image/png" {
		t.Errorf("the guard saw %+v, want the page with its facts, then the attachment with its file", g.steps)
	}
	second, err := f.importAs(first, func(ctx context.Context, u *app.ImportUnit) error {
		_, err := u.CreatePage(ctx, app.ImportedPage{Name: "More"})
		return err
	})
	if err != nil || second != first || len(f.store.changesets) != 1 {
		t.Errorf("a later unit = %s, %v, changesets %+v; want it merged into %s", second, err, f.store.changesets, first)
	}
}

// A name a sibling holds, by key, is numbered: among the parent's
// children before the import and those the unit created; an attachment's
// number goes before its extension.
func TestAnImportsUnitNumbersANameTaken(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	f.page("Notes", nil, 0)
	f.asset("photo.png", nil, 1)
	f.asset("PHOTO 2.png", nil, 2)
	var got []string
	_, err := f.importAs(uuid.UUID{}, func(ctx context.Context, u *app.ImportUnit) error {
		for _, name := range []string{"notes", "NOTES", "Straße", "STRASSE"} {
			n, err := u.CreatePage(ctx, app.ImportedPage{Name: name})
			if err != nil {
				return err
			}
			got = append(got, n.Name)
		}
		for _, name := range []string{"photo.png", "Photo.PNG", "README", "readme"} {
			n, err := u.CreateAsset(ctx, app.ImportedAsset{Name: name}, func(context.Context, domain.Node) error { return nil })
			if err != nil {
				return err
			}
			got = append(got, n.Name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"notes 2", "NOTES 3", "Straße", "STRASSE 2", "photo 3.png", "Photo 4.PNG", "README", "readme 2"}
	if !slices.Equal(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

// A name numbered takes none reserved, a later sibling's of the import;
// its own name it keeps, reserved or not. Each name, in any spelling of
// its key, takes the numbers after the last one of them took: none is
// tried twice.
func TestAnImportsUnitNumbersPastTheReservedNames(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	f.page("Untitled", nil, 0)
	asked := map[string]int{}
	reserved := func(key string) bool {
		asked[key]++
		return key == "untitled 2" || key == "untitled 3.png" || key == "x"
	}
	var got []string
	_, err := f.importAs(uuid.UUID{}, func(ctx context.Context, u *app.ImportUnit) error {
		for _, name := range []string{"Untitled", "Untitled", "UNTITLED", "x", "x"} {
			n, err := u.CreatePage(ctx, app.ImportedPage{Name: name, Reserved: reserved})
			if err != nil {
				return err
			}
			got = append(got, n.Name)
		}
		for range 3 {
			n, err := u.CreateAsset(ctx, app.ImportedAsset{Name: "untitled.png", Reserved: reserved},
				func(context.Context, domain.Node) error { return nil })
			if err != nil {
				return err
			}
			got = append(got, n.Name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Untitled 3", "Untitled 4", "UNTITLED 5", "x", "x 2", "untitled.png", "untitled 2.png", "untitled 4.png"}
	if !slices.Equal(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
	for key, n := range asked {
		if n > 1 {
			t.Errorf("%q asked %d times, want each number tried once", key, n)
		}
	}
}

// A name the import did not mend is refused, 422, as a unit refuses it.
func TestAnImportsUnitRefusesANameNotMended(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	for _, do := range []func(ctx context.Context, u *app.ImportUnit) error{
		func(ctx context.Context, u *app.ImportUnit) error {
			_, err := u.CreatePage(ctx, app.ImportedPage{Name: "a/b"})
			return err
		},
		func(ctx context.Context, u *app.ImportUnit) error {
			_, err := u.CreateAsset(ctx, app.ImportedAsset{Name: "notes.md"}, func(context.Context, domain.Node) error { return nil })
			return err
		},
	} {
		if _, err := f.importAs(uuid.UUID{}, do); codeOf(err) != "validation_failed name" {
			t.Errorf("an import's unit of a name not mended = %v, want 422", err)
		}
	}
}

// A page deeper than pages go is refused before anything is written: the
// unit goes on with the rest, attachments under a page as deep as pages go
// among them.
func TestAnImportsUnitRefusesAPageTooDeep(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	var parent *uuid.UUID
	for i := range domain.MaxDepth {
		n := f.page("Level "+strings.Repeat("i", i+1), parent, 0)
		parent = &n.ID
	}
	f.rec.calls = nil
	_, err := f.importAs(uuid.UUID{}, func(ctx context.Context, u *app.ImportUnit) error {
		if _, err := u.CreatePage(ctx, app.ImportedPage{ParentID: parent, Name: "Too deep"}); !errors.Is(err, domain.ErrTooDeep) {
			return errors.New("not refused too deep: " + codeOf(err))
		}
		if _, err := u.CreateAsset(ctx, app.ImportedAsset{ParentID: parent, Name: "deep.png"},
			func(context.Context, domain.Node) error { return nil }); err != nil {
			return err
		}
		_, err := u.CreatePage(ctx, app.ImportedPage{Name: "Shallow"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.childNames(parent); !slices.Equal(got, []string{"deep.png"}) {
		t.Errorf("the deepest page's children = %v, want the attachment alone", got)
	}
	if got := f.childNames(nil); !slices.Equal(got, []string{"Level i", "Shallow"}) {
		t.Errorf("the root's children = %v, want the shallow page created", got)
	}
}

// A parent that is no page of the notebook is ErrNoParent: deleted, an
// attachment, another notebook's.
func TestAnImportsUnitRefusesAParentGone(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	a := f.asset("photo.png", nil, 0)
	other := f.page("Elsewhere", nil, 1)
	other.NotebookID = uuid.NewV7()
	f.store.nodes[other.ID] = other
	gone := uuid.NewV7()
	for _, parent := range []uuid.UUID{a.ID, other.ID, gone} {
		_, err := f.importAs(uuid.UUID{}, func(ctx context.Context, u *app.ImportUnit) error {
			_, err := u.CreatePage(ctx, app.ImportedPage{ParentID: &parent, Name: "Child"})
			return err
		})
		if !errors.Is(err, app.ErrNoParent) {
			t.Errorf("a page under %s = %v, want ErrNoParent", parent, err)
		}
		_, err = f.importAs(uuid.UUID{}, func(ctx context.Context, u *app.ImportUnit) error {
			_, err := u.CreateAsset(ctx, app.ImportedAsset{ParentID: &parent, Name: "c.png"}, func(context.Context, domain.Node) error { return nil })
			return err
		})
		if !errors.Is(err, app.ErrNoParent) {
			t.Errorf("an attachment under %s = %v, want ErrNoParent", parent, err)
		}
	}
}

// A unit reads each parent's children and line once, those it creates
// added to them; siblings renumbered as one is added keep their new
// orders for the next.
func TestAnImportsUnitReadsEachParentOnce(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionCreate)
	top := f.page("Top", nil, 0)
	// No double past 2^53 follows it: the next appended renumbers them.
	f.page("Huge", &top.ID, 1<<53)
	f.rec.calls = nil
	_, err := f.importAs(uuid.UUID{}, func(ctx context.Context, u *app.ImportUnit) error {
		for _, name := range []string{"a", "b", "c"} {
			if _, err := u.CreatePage(ctx, app.ImportedPage{ParentID: &top.ID, Name: name}); err != nil {
				return err
			}
		}
		for _, name := range []string{"a", "x.png"} {
			if _, err := u.CreateAsset(ctx, app.ImportedAsset{ParentID: &top.ID, Name: name},
				func(context.Context, domain.Node) error { return nil }); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	count := func(call string) int {
		n := 0
		for _, c := range f.rec.calls {
			if strings.HasPrefix(c, call) {
				n++
			}
		}
		return n
	}
	if count("Children") != 1 || count("Ancestors") != 1 {
		t.Errorf("calls = %v, want the children and the line read once", f.rec.calls)
	}
	if got := f.childNames(&top.ID); !slices.Equal(got, []string{"Huge", "a", "b", "c", "a 2", "x.png"}) {
		t.Errorf("Top's children = %v, want the import's after Huge, in order", got)
	}
}
