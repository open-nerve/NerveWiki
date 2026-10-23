package app_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Errors the fake paths answer, as page's would in its tools' words: the
// use cases answer them as they are.
var (
	errNothingAtPath = errors.New("page.not_found: nothing at the path")
	errAttachment    = errors.New("page.not_found: an attachment's path")
)

// paths is the page module's paths, faked: each node alive by its path,
// a page's ending with .md; whether it is an attachment, its parent, when
// its content last changed. Its calls go to the library's.
type paths struct {
	*library
	byPath  map[string]uuid.UUID
	at      map[uuid.UUID]string
	assets  map[uuid.UUID]bool
	parents map[uuid.UUID]uuid.UUID
	changed map[uuid.UUID]time.Time
}

func newPaths(l *library) *paths {
	return &paths{library: l, byPath: map[string]uuid.UUID{}, at: map[uuid.UUID]string{}, assets: map[uuid.UUID]bool{},
		parents: map[uuid.UUID]uuid.UUID{}, changed: map[uuid.UUID]time.Time{}}
}

// add puts id at path, under the node at its parent's path, the root for
// none; an attachment when its path does not end with .md.
func (p *paths) add(id uuid.UUID, path string) uuid.UUID {
	p.byPath[path], p.at[id] = id, path
	p.assets[id] = !strings.HasSuffix(path, ".md")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		p.parents[id] = p.byPath[path[:i]+".md"]
	}
	p.changed[id] = time.Date(2026, 10, 1, 0, 0, len(p.at), 0, time.UTC)
	return id
}

// gone puts the node id in the trash: it has no path.
func (p *paths) gone(id uuid.UUID) {
	delete(p.byPath, p.at[id])
	delete(p.at, id)
}

func (p *paths) Check(field, path string) error {
	p.call("Check " + field)
	if path == "" || strings.Contains(path, "//") || strings.ContainsRune(path, 0) {
		return shared.Invalid(shared.FieldError{Field: field, Code: shared.FieldInvalidFormat, Message: "is no path"})
	}
	return nil
}

func (p *paths) FindPage(_ context.Context, _ uuid.UUID, field, path string) (uuid.UUID, error) {
	p.call("FindPage " + field + " " + path)
	id, ok := p.byPath[path]
	switch {
	case !ok:
		return uuid.UUID{}, errNothingAtPath
	case p.assets[id]:
		return uuid.UUID{}, errAttachment
	}
	return id, nil
}

func (p *paths) Paths(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	p.call("NodePaths")
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if at, ok := p.at[id]; ok {
			out[id] = at
		}
	}
	return out, nil
}

func (p *paths) Pages(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]app.PagePath, error) {
	p.call("Pages")
	out := map[uuid.UUID]app.PagePath{}
	for _, id := range ids {
		if at, ok := p.at[id]; ok && !p.assets[id] {
			out[id] = app.PagePath{Path: at, ChangedAt: p.changed[id]}
		}
	}
	return out, nil
}

func (p *paths) ChildPages(_ context.Context, _, parent uuid.UUID) ([]uuid.UUID, error) {
	p.call("ChildPages")
	var out []uuid.UUID
	for id := range p.at {
		if !p.assets[id] && p.parents[id] == parent {
			out = append(out, id)
		}
	}
	return out, nil
}

// backlinksAt is the library's ListBacklinks over paths.
func (l lib) backlinksAt(p *paths) app.ListBacklinks {
	b := l.listBacklinks()
	b.Paths = p
	return b
}

// MCP's backlinks judge the cursor, the path, the limit, the notebook and
// the page at the path, in this order, each before any read the next
// needs; a path that names nothing and an attachment's are page's errors,
// as they are (M9/P4 design 3.9.2; 2.1, item 16).
func TestListBacklinksAtPathJudgesInTurn(t *testing.T) {
	l := newLibrary()
	p := newPaths(l.library)
	p.add(l.p, "P.md")
	p.add(uuid.NewV7(), "P/x.png")
	bad, zero := "x", 0
	for _, tt := range []struct {
		name   string
		nb     uuid.UUID
		path   string
		limit  *int
		cursor *string
		is     error
		field  string
		calls  []string
	}{
		{name: "a bad cursor", nb: l.nb, path: "a//b.md", limit: &zero, cursor: &bad, is: shared.InvalidCursor()},
		{name: "a bad path", nb: l.hidden, path: "a//b.md", limit: &zero, field: "path", calls: []string{"Check path"}},
		{name: "a bad limit", nb: l.hidden, path: "P.md", limit: &zero, field: "limit", calls: []string{"Check path"}},
		{name: "a notebook hidden", nb: l.hidden, path: "P.md", is: domain.ErrNotebookNotFound,
			calls: []string{"Check path", "WorkspaceOf", "Authorize backlink.list"}},
		{name: "no notebook", nb: uuid.NewV7(), path: "P.md", is: domain.ErrNotebookNotFound,
			calls: []string{"Check path", "WorkspaceOf"}},
		{name: "nothing at the path", nb: l.nb, path: "Q.md", is: errNothingAtPath,
			calls: []string{"Check path", "WorkspaceOf", "Authorize backlink.list", "FindPage path Q.md"}},
		{name: "an attachment's path", nb: l.nb, path: "P/x.png", is: errAttachment,
			calls: []string{"Check path", "WorkspaceOf", "Authorize backlink.list", "FindPage path P/x.png"}},
	} {
		l.calls = nil
		_, err := l.backlinksAt(p).AtPath(reader(), tt.nb, tt.path, tt.limit, tt.cursor)
		var problem *shared.Error
		switch {
		case tt.is != nil && !errors.Is(err, tt.is):
			t.Errorf("%s: %v, want %v", tt.name, err, tt.is)
		case tt.field != "" && (!errors.As(err, &problem) || problem.Code != shared.CodeValidationFailed ||
			len(problem.Fields) != 1 || problem.Fields[0].Field != tt.field):
			t.Errorf("%s: %v, want 422 at %s", tt.name, err, tt.field)
		}
		if !slices.Equal(l.calls, tt.calls) {
			t.Errorf("%s: calls %v, want %v", tt.name, l.calls, tt.calls)
		}
	}
	if _, err := l.backlinksAt(p).AtPath(context.Background(), l.nb, "P.md", nil, nil); !errors.Is(err, shared.Unauthenticated()) {
		t.Errorf("without an actor: %v", err)
	}
}

// MCP's backlinks are Execute's of the page at the path, each source with
// its path now and the cursor after it; a source put in the trash since
// the rows were read is left out, and the next page's cursor goes past
// it, the last of a page among them; the sources' paths are one read
// (M9/P4 design 3.9.2; 2.1, item 18).
func TestListBacklinksAtPath(t *testing.T) {
	l := newLibrary()
	p := newPaths(l.library)
	p.add(l.p, "P.md")
	ids := []uuid.UUID{uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()}
	slices.SortFunc(ids, uuid.UUID.Compare)
	p.add(ids[0], "A.md")
	p.add(ids[1], "A/B.md")
	p.add(ids[2], "C.md")
	p.add(ids[3], "D.md")
	p.gone(ids[1])
	p.gone(ids[3])
	e := domain.Extractor
	l.contents = map[uuid.UUID]revised{ids[0]: {"[[P]]", 1}, ids[2]: {"x [[P]]\ny [[P]]", 2}}
	l.backlinks = []app.Backlink{
		{SourceID: ids[0], Revision: 1, Extractor: e, Links: 1, Ranges: []domain.Range{{Start: 2, End: 3}}},
		{SourceID: ids[1], Revision: 1, Extractor: e, Links: 1, Ranges: []domain.Range{{Start: 2, End: 3}}},
		{SourceID: ids[2], Revision: 2, Extractor: e, Links: 2, Ranges: []domain.Range{{Start: 4, End: 5}, {Start: 12, End: 13}}},
		{SourceID: ids[3], Revision: 1, Extractor: e, Links: 1, Ranges: []domain.Range{{Start: 2, End: 3}}},
	}
	cursorOf := func(id uuid.UUID) string {
		c, err := shared.EncodeCursor(id)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}

	two := 2
	first, err := l.backlinksAt(p).AtPath(reader(), l.nb, "P.md", &two, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := app.BacklinksAt{Pages: []app.BacklinkingAt{{
		Backlinking: app.Backlinking{PageID: ids[0], Links: 1, Contexts: []string{"[[P]]"}}, Path: "A.md", Cursor: cursorOf(ids[0]),
	}}, NextCursor: cursorOf(ids[1])}
	if !reflect.DeepEqual(first, want) {
		t.Errorf("the first page, its last gone: %+v\nwant %+v", first, want)
	}
	if want := []string{"Check path", "WorkspaceOf", "Authorize backlink.list", "FindPage path P.md", "Backlinks", "Content", "Content", "NodePaths"}; !slices.Equal(l.calls, want) {
		t.Errorf("calls %v, want %v", l.calls, want)
	}
	second, err := l.backlinksAt(p).AtPath(reader(), l.nb, "P.md", &two, &first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	want = app.BacklinksAt{Pages: []app.BacklinkingAt{{
		Backlinking: app.Backlinking{PageID: ids[2], Links: 2, Contexts: []string{"x [[P]]", "y [[P]]"}}, Path: "C.md", Cursor: cursorOf(ids[2]),
	}}}
	if !reflect.DeepEqual(second, want) {
		t.Errorf("the last page, its last gone: %+v\nwant %+v", second, want)
	}
	if got, err := l.backlinksAt(p).AtPath(reader(), l.nb, "P.md", nil, &first.Pages[0].Cursor); err != nil ||
		len(got.Pages) != 1 || got.Pages[0].PageID != ids[2] || got.NextCursor != "" {
		t.Errorf("from the first item's cursor: %+v, %v", got, err)
	}
}
